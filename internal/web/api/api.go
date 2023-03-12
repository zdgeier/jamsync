package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/server/client"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

func UserProjectsHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		resp, err := tempClient.ListUserProjects(ctx, &pb.ListUserProjectsRequest{})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.JSON(200, resp)
	}
}

func ListCommittedChanges() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		resp, err := tempClient.ListCommittedChanges(ctx, &pb.ListCommittedChangesRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.Error(err)
			return
		}
		ctx.JSON(200, resp)
	}
}

func ProjectBrowseHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		config, err := tempClient.GetProjectConfig(ctx, &pb.GetProjectConfigRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		changeId, err := strconv.Atoi(ctx.Query("commitId"))
		if err != nil {
			ctx.String(http.StatusBadRequest, err.Error())
			return
		}

		client := client.NewClient(tempClient, config.GetProjectId(), uint64(changeId))
		resp, err := client.BrowseProject(ctx.Param("path")[1:])
		if err != nil {
			ctx.Error(err)
			return
		}
		ctx.JSON(200, resp)
	}
}

func GetFileHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		changeId, err := strconv.Atoi(ctx.Query("commitId"))
		if err != nil {
			ctx.String(http.StatusBadRequest, err.Error())
			return
		}

		config, err := tempClient.GetProjectConfig(ctx, &pb.GetProjectConfigRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.Error(err)
			return
		}

		client := client.NewClient(tempClient, config.GetProjectId(), uint64(changeId))

		client.DownloadFile(ctx, ctx.Param("path")[1:], bytes.NewReader([]byte{}), ctx.Writer)
	}
}

func PutFileHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		config, err := tempClient.GetProjectConfig(ctx, &pb.GetProjectConfigRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		client := client.NewClient(tempClient, config.GetProjectId(), config.GetCurrentChange())

		data, err := io.ReadAll(ctx.Request.Body)
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.Request.Body.Close()

		err = client.UpdateFile(ctx, ctx.Param("path")[1:], data)
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		type PutFileResponse struct {
			CommitId uint64 `json:"commitId"`
		}

		ctx.JSON(http.StatusCreated, PutFileResponse{
			CommitId: client.ProjectConfig().CurrentChange,
		})
	}
}

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

func CommitChangeWSHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		config, err := tempClient.GetProjectConfig(ctx, &pb.GetProjectConfigRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		conn, err := wsupgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer conn.Close()

		changeStream, err := tempClient.ChangeStream(ctx, &pb.ChangeStreamRequest{ProjectId: config.ProjectId})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer changeStream.CloseSend()

		for {
			recv, err := changeStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
				break
			}

			resp, err := json.Marshal(recv)
			if err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
				return
			}
			conn.WriteMessage(websocket.TextMessage, []byte(resp))
		}
	}

}

func EditorChangeWSHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		fmt.Println("test")
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		config, err := tempClient.GetProjectConfig(ctx, &pb.GetProjectConfigRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		conn, err := wsupgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer conn.Close()

		fmt.Println("here")
		editorStream, err := tempClient.EditorStream(ctx)
		if err != nil {
			log.Println(err)
			return
		}
		defer editorStream.CloseSend()

		editorStream.Send(&pb.EditorStreamMessage{ProjectId: config.GetProjectId()})

		go func() {
			for {
				_, data, err := conn.ReadMessage()
				if err != nil {
					log.Println(err)
					break
				}
				editorStream.Send(&pb.EditorStreamMessage{
					ProjectId:  config.ProjectId,
					UpdateData: data,
				})
			}
		}()

		for {
			fmt.Println("wating")
			recv, err := editorStream.Recv()
			if err == io.EOF {
				fmt.Println("out", recv)
				break
			}
			if err != nil {
				fmt.Println("err", err)
				log.Println(err)
				break
			}
			fmt.Println("RECV", recv)

			conn.WriteMessage(websocket.TextMessage, recv.UpdateData)
		}
	}
}
