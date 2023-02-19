package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"log"
	"net"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/jamenv"
	"github.com/zdgeier/jamsync/internal/server/changestore"
	"github.com/zdgeier/jamsync/internal/server/db"
	"github.com/zdgeier/jamsync/internal/server/hub"
	"github.com/zdgeier/jamsync/internal/server/oplocstore"
	"github.com/zdgeier/jamsync/internal/server/opstore"
	"github.com/zdgeier/jamsync/internal/server/serverauth"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/reflection"
)

//go:embed clientkey.pem
var prodF embed.FS

//go:embed devclientkey.cer
var devF embed.FS

type JamsyncServer struct {
	db          db.JamsyncDb
	opstore     opstore.LocalStore
	oplocstore  oplocstore.LocalOpLocStore
	changestore changestore.LocalChangeStore
	hub         hub.Hub
	pb.UnimplementedJamsyncAPIServer
}

func New() (closer func(), err error) {
	jamsyncServer := JamsyncServer{
		db:          db.New(),
		opstore:     opstore.NewLocalStore("jb"),
		oplocstore:  oplocstore.NewLocalOpLocStore("jb"),
		changestore: changestore.NewLocalChangeStore(),
		hub:         *hub.NewHub(),
	}

	var cert tls.Certificate
	if jamenv.Env() == jamenv.Prod {
		cert, err = tls.LoadX509KeyPair("/etc/jamsync/fullchain.pem", "/etc/jamsync/privkey.pem")
	} else {
		cert, err = tls.LoadX509KeyPair("/etc/jamsync/x509/publickey.cer", "/etc/jamsync/x509/private.key")
	}
	if err != nil {
		return nil, err
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(serverauth.EnsureValidToken),
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
	}

	server := grpc.NewServer(opts...)
	reflection.Register(server)
	pb.RegisterJamsyncAPIServer(server, jamsyncServer)

	tcplis, err := net.Listen("tcp", "0.0.0.0:14357")
	if err != nil {
		return nil, err
	}
	go func() {
		if err := server.Serve(tcplis); err != nil {
			log.Printf("error serving server: %v", err)
		}
	}()

	go jamsyncServer.hub.Run()

	return func() { server.Stop() }, nil
}

func Connect(accessToken *oauth2.Token) (client pb.JamsyncAPIClient, closer func(), err error) {
	opts := []grpc.DialOption{
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			raddr, err := net.ResolveTCPAddr("tcp", addr)
			if err != nil {
				return nil, err
			}

			conn, err := net.DialTCP("tcp", nil, raddr)
			if err != nil {
				return nil, err
			}

			return conn, err
		}),
	}
	if jamenv.Env() != jamenv.Local {
		perRPC := oauth.TokenSource{TokenSource: oauth2.StaticTokenSource(accessToken)}
		opts = append(opts, grpc.WithPerRPCCredentials(perRPC))
	}
	var creds credentials.TransportCredentials
	if jamenv.Env() == jamenv.Prod {
		cp := x509.NewCertPool()
		certData, err := prodF.ReadFile("clientkey.pem")
		if err != nil {
			return nil, nil, err
		}
		cp.AppendCertsFromPEM(certData)
		creds = credentials.NewClientTLSFromCert(cp, "jamsync.dev")
	} else {
		cp := x509.NewCertPool()
		certData, err := devF.ReadFile("devclientkey.cer")
		if err != nil {
			return nil, nil, err
		}
		cp.AppendCertsFromPEM(certData)
		creds = credentials.NewClientTLSFromCert(cp, "jamsync.dev")
	}
	opts = append(opts, grpc.WithTransportCredentials(creds))

	addr := "0.0.0.0:14357"
	if jamenv.Env() == jamenv.Prod {
		addr = "jamsync.dev:14357"
	}
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		log.Panicf("could not connect to jamsync server: %s", err)
	}
	client = pb.NewJamsyncAPIClient(conn)
	closer = func() {
		if err := conn.Close(); err != nil {
			log.Panic("could not close server connection")
		}
	}

	return client, closer, err
}
