package server

import (
	"context"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/server/serverauth"
)

func (s JamsyncServer) CreateUser(ctx context.Context, in *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	err = s.db.CreateUser(in.GetUsername(), id)
	if err != nil {
		return nil, err
	}
	return &pb.CreateUserResponse{}, nil
}

func (s JamsyncServer) Ping(ctx context.Context, in *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{}, nil
}
