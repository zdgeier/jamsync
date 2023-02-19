package server

import (
	"context"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/server/serverauth"
)

func (s JamsyncServer) AddProject(ctx context.Context, in *pb.AddProjectRequest) (*pb.AddProjectResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	projectId, err := s.db.AddProject(in.GetProjectName(), id)
	if err != nil {
		return nil, err
	}

	return &pb.AddProjectResponse{
		ProjectId: projectId,
	}, nil
}

func (s JamsyncServer) ListUserProjects(ctx context.Context, in *pb.ListUserProjectsRequest) (*pb.ListUserProjectsResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	projects, err := s.db.ListUserProjects(id)
	if err != nil {
		return nil, err
	}

	projectsPb := make([]*pb.ListUserProjectsResponse_Project, len(projects))
	for i := range projectsPb {
		projectsPb[i] = &pb.ListUserProjectsResponse_Project{Name: projects[i].Name, Id: projects[i].Id}
	}

	return &pb.ListUserProjectsResponse{Projects: projectsPb}, nil
}

func (s JamsyncServer) ListProjects(ctx context.Context, in *pb.ListProjectsRequest) (*pb.ListProjectsResponse, error) {
	projects, err := s.db.ListProjects()
	if err != nil {
		return nil, err
	}

	projectsPb := make([]*pb.ListProjectsResponse_Project, len(projects))
	for i := range projectsPb {
		projectsPb[i] = &pb.ListProjectsResponse_Project{Name: projects[i].Name, Id: projects[i].Id}
	}

	return &pb.ListProjectsResponse{Projects: projectsPb}, nil
}

func (s JamsyncServer) GetProjectConfig(ctx context.Context, in *pb.GetProjectConfigRequest) (*pb.ProjectConfig, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	var projectId uint64
	if in.GetProjectName() == "" && in.GetProjectId() != 0 {
		projectId = in.GetProjectId()
	} else {
		projectId, err = s.db.GetProjectId(in.GetProjectName(), userId)
		if err != nil {
			return nil, err
		}
	}

	changeId, _, err := s.changestore.GetCurrentChange(projectId, userId)
	if err != nil {
		return nil, err
	}

	return &pb.ProjectConfig{ProjectId: projectId, CurrentChange: changeId}, nil
}
