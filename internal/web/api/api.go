package api

import (
	"bytes"
	"fmt"
	"io"
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

func ProjectsHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		resp, err := tempClient.ListProjects(ctx, &pb.ListProjectsRequest{})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.JSON(200, resp)
	}
}

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
		err = client.UpdateFile(ctx, ctx.Param("path")[1:], ctx.Request.Body)
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
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
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

		changeStream, err := tempClient.ChangeStream(ctx, &pb.ChangeStreamRequest{ProjectId: config.ProjectId})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer changeStream.CloseSend()

		for {
			_, err := changeStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
				break
			}

			config, err := tempClient.GetProjectConfig(ctx, &pb.GetProjectConfigRequest{
				ProjectName: ctx.Param("projectName"),
			})
			if err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
				return
			}
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprint(config.CurrentChange)))
		}
	}

}
