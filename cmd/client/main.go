package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cespare/xxhash"
	"github.com/fsnotify/fsnotify"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/jamenv"
	jam "github.com/zdgeier/jamsync/internal/server/client"
	"github.com/zdgeier/jamsync/internal/server/clientauth"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	version string
	built   string
)

func main() {
	log.Println("version: " + version)
	log.Println("built: " + built)
	log.Println("env: " + jamenv.Env().String())
	accessToken, err := clientauth.InitConfig()
	if err != nil {
		log.Panic(err)
	}
	apiClient, closer, err := server.Connect(&oauth2.Token{
		AccessToken: accessToken,
	})
	if err != nil {
		log.Panic(err)
	}
	defer closer()

	pingRes, err := apiClient.Ping(context.Background(), &pb.PingRequest{})
	if err != nil {
		fmt.Println(err)
		accessToken, err := clientauth.ReauthConfig()
		if err != nil {
			log.Panic(err)
		}
		apiClient, closer, err = server.Connect(&oauth2.Token{
			AccessToken: accessToken,
		})
		if err != nil {
			log.Panic(err)
		}
		defer closer()
	}
	if jamenv.Env() == jamenv.Prod {
		log.Println("user: ", pingRes.Username)
	}

	currentPath, err := os.Getwd()
	if err != nil {
		log.Panic(err)
	}

	empty, err := currentDirectoryEmpty()
	if err != nil {
		log.Panic(err)
	}

	var client *jam.Client
	if empty {
		log.Println("This directory is empty.")
		log.Print("Name of project to download: ")
		var projectName string
		fmt.Scan(&projectName)

		resp, err := apiClient.GetProjectConfig(context.Background(), &pb.GetProjectConfigRequest{
			ProjectName: projectName,
		})
		if err != nil {
			log.Panic(err)
		}

		client = jam.NewClient(apiClient, resp.ProjectId, resp.CurrentChange)

		diffRemoteToLocalResp, err := client.DiffRemoteToLocal(context.Background(), &pb.FileMetadata{})
		if err != nil {
			log.Panic(err)
		}

		err = applyFileListDiff(diffRemoteToLocalResp, client)
		if err != nil {
			log.Panic(err)
		}

		log.Println("Done downloading.")
		err = writeJamsyncFile(client.ProjectConfig())
		if err != nil {
			log.Panic(err)
		}
	} else if config := findJamsyncConfig(); config != nil {
		client = jam.NewClient(apiClient, config.ProjectId, config.CurrentChange)
	} else {
		log.Println("This directory has some existing contents.")
		log.Print("Name of new project to create for current directory: ")
		var projectName string
		fmt.Scan(&projectName)

		resp, err := apiClient.AddProject(context.Background(), &pb.AddProjectRequest{
			ProjectName: projectName,
		})
		if err != nil {
			log.Panic(err)
		}
		log.Println("Initializing a project at " + currentPath)

		client = jam.NewClient(apiClient, resp.ProjectId, 0)
		err = uploadNewProject(client)
		if err != nil {
			log.Panic(err)
		}
	}

	// Get what has changed locally since last push
	fileMetadata := readLocalFileList()
	localToRemoteDiff, err := client.DiffLocalToRemote(context.Background(), fileMetadata)
	if err != nil {
		log.Panic(err)
	}

	remoteConfig, err := apiClient.GetProjectConfig(context.Background(), &pb.GetProjectConfigRequest{
		ProjectId: client.ProjectConfig().GetProjectId(),
	})
	if err != nil {
		log.Panic(err)
	}

	if client.ProjectConfig().CurrentChange == remoteConfig.CurrentChange {
		if diffHasChanges(localToRemoteDiff) {
			err = pushFileListDiff(fileMetadata, localToRemoteDiff, client)
			if err != nil {
				log.Panic(err)
			}
			writeJamsyncFile(client.ProjectConfig())
		}

		stream, err := apiClient.ChangeStream(context.Background(), &pb.ChangeStreamRequest{
			ProjectId: client.ProjectConfig().GetProjectId(),
		})
		if err != nil {
			log.Panic(err)
		}
		changes := make(chan *pb.ChangeStreamMessage)
		go func() {
			for {
				in, err := stream.Recv()
				if err == io.EOF {
					log.Println("Stopped change stream")
					return
				}
				if err != nil {
					log.Fatalf("Failed to receive a change stream message: %v", err)
				}
				changes <- in
			}
		}()

		watcher, _ := fsnotify.NewWatcher()
		defer watcher.Close()

		if err := filepath.WalkDir(".", func(path string, d fs.DirEntry, _ error) error {
			if shouldExclude(path) {
				return nil
			}
			log.Println("Watching", path)
			return watcher.Add(path)
		}); err != nil {
			log.Panic("Could not walk directory tree to watch files", err)
		}

		for {
			select {
			case <-changes:
				log.Println("Got remote change")
				fileMetadata := readLocalFileList()
				localToRemoteDiff, err := client.DiffLocalToRemote(context.Background(), fileMetadata)
				if err != nil {
					log.Panic(err)
				}

				remoteConfig, err := apiClient.GetProjectConfig(context.Background(), &pb.GetProjectConfigRequest{
					ProjectId: client.ProjectConfig().GetProjectId(),
				})
				if err != nil {
					log.Panic(err)
				}
				client = jam.NewClient(apiClient, remoteConfig.ProjectId, remoteConfig.CurrentChange)
				remoteToLocalDiff, err := client.DiffRemoteToLocal(context.Background(), fileMetadata)
				if err != nil {
					log.Panic(err)
				}

				pull(client, localToRemoteDiff, remoteToLocalDiff)
			case event := <-watcher.Events:
				if event.Op == fsnotify.Chmod {
					continue
				}

				path := event.Name

				if strings.HasSuffix(path, ".jamtemp") {
					continue
				}

				if stat, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
					log.Println(path + " deleted")
					err := watcher.Remove(path)
					if err != nil {
						log.Fatal(err)
					}
				} else if stat.IsDir() {
					if err := filepath.WalkDir(path, func(path string, d fs.DirEntry, _ error) error {
						if d.IsDir() {
							log.Println(path + " directory changed")
						} else {
							log.Println(path + " changed")
						}

						return watcher.Add(path)
					}); err != nil {
						log.Panic("Could not walk directory tree to watch files")
					}
				} else {
					log.Println(path + " file changed")
					err := watcher.Add(path)
					if err != nil {
						log.Fatal(err)
					}
				}
				fileMetadata := readLocalFileList()
				localToRemoteDiff, err := client.DiffLocalToRemote(context.Background(), fileMetadata)
				if err != nil {
					log.Panic(err)
				}
				if diffHasChanges(localToRemoteDiff) {
					err = pushFileListDiff(fileMetadata, localToRemoteDiff, client)
					if err != nil {
						log.Panic(err)
					}
					writeJamsyncFile(client.ProjectConfig())
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	} else {
		client = jam.NewClient(apiClient, remoteConfig.ProjectId, remoteConfig.CurrentChange)
		remoteToLocalDiff, err := client.DiffRemoteToLocal(context.Background(), fileMetadata)
		if err != nil {
			log.Panic(err)
		}

		pull(client, localToRemoteDiff, remoteToLocalDiff)
	}
}

func diffHasChanges(diff *pb.FileMetadataDiff) bool {
	for _, diff := range diff.GetDiffs() {
		if diff.Type != pb.FileMetadataDiff_NoOp {
			return true
		}
	}
	return false
}

func pull(client *jam.Client, localToRemoteDiff *pb.FileMetadataDiff, remoteToLocalDiff *pb.FileMetadataDiff) {
	if err := filepath.WalkDir(".", func(path string, d fs.DirEntry, _ error) error {
		if !d.IsDir() {
			if strings.HasSuffix(path, ".jamdiff") {
				return fmt.Errorf(".jamdiff file found at %s", path)
			}
		}
		return nil
	}); err != nil {
		log.Println(err)
		return
	}

	dirty := false
	for path, remoteDiff := range remoteToLocalDiff.GetDiffs() {
		if remoteDiff.GetType() != pb.FileMetadataDiff_NoOp {
			// Local has changed
			if localDiff, found := localToRemoteDiff.GetDiffs()[path]; found && localDiff.GetType() != pb.FileMetadataDiff_NoOp {
				if localDiff.GetFile().Hash == remoteDiff.GetFile().Hash {
					newModTime := remoteDiff.File.GetModTime().AsTime()
					err := os.Chtimes(path, newModTime, newModTime)
					if err != nil {
						log.Panic(err)
					}
					continue
				}
				file, err := os.OpenFile(path+".jamdiff", os.O_RDWR|os.O_CREATE, 0755)
				if err != nil {
					log.Panic(err)
				}

				reader, err := os.Open(path)
				if err != nil {
					log.Panic(err)
				}

				err = client.DownloadFile(context.Background(), path, reader, file)
				if err != nil {
					log.Panic(err)
				}
				newModTime := remoteDiff.File.GetModTime().AsTime()
				err = os.Chtimes(path, newModTime, newModTime)
				if err != nil {
					log.Panic(err)
				}
				dirty = true
			}
		}
	}

	if dirty {
		writeJamsyncFile(client.ProjectConfig())
		log.Println("merge .jamdiff files to continue")
		return
	}

	err := applyFileListDiff(remoteToLocalDiff, client)
	if err != nil {
		log.Panic(err)
	}

	log.Println("Done downloading.")
}

func findJamsyncConfig() *pb.ProjectConfig {
	currentPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		filePath := fmt.Sprintf("%v/%v", currentPath, ".jamsync")
		_, err := os.Stat(filePath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			panic(err)
		} else if err == nil {
			if configBytes, err := os.ReadFile(filePath); err == nil {
				config := &pb.ProjectConfig{}
				err = proto.Unmarshal(configBytes, config)
				if err != nil {
					panic(err)
				}
				return config
			}
		} else if currentPath == "/" {
			break
		}
		currentPath = path.Dir(currentPath)
	}
	return nil
}

func writeJamsyncFile(config *pb.ProjectConfig) error {
	f, err := os.Create(".jamsync")
	if err != nil {
		return err
	}
	defer f.Close()

	configBytes, err := proto.Marshal(config)
	if err != nil {
		return err
	}
	_, err = f.Write(configBytes)
	return err
}

func uploadNewProject(client *jam.Client) error {
	fileMetadata := readLocalFileList()
	fileMetadataDiff, err := client.DiffLocalToRemote(context.Background(), fileMetadata)
	if err != nil {
		return err
	}
	err = pushFileListDiff(fileMetadata, fileMetadataDiff, client)
	if err != nil {
		return err
	}
	return writeJamsyncFile(client.ProjectConfig())
}

func readLocalFileList() *pb.FileMetadata {
	files := map[string]*pb.File{}
	if err := filepath.WalkDir(".", func(path string, d fs.DirEntry, _ error) error {
		if shouldExclude(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			files[path] = &pb.File{
				ModTime: timestamppb.New(info.ModTime()),
				Dir:     true,
			}
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			h := xxhash.New()
			h.Write(data)

			files[path] = &pb.File{
				ModTime: timestamppb.New(info.ModTime()),
				Dir:     false,
				Hash:    h.Sum64(),
			}
		}
		return nil
	}); err != nil {
		log.Println("WARN: could not walk directory tree", err)
	}

	return &pb.FileMetadata{
		Files: files,
	}
}

func pushFileListDiff(fileMetadata *pb.FileMetadata, fileMetadataDiff *pb.FileMetadataDiff, client *jam.Client) error {
	ctx := context.Background()

	err := client.CreateChange()
	if err != nil {
		return err
	}

	for path, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && diff.GetType() != pb.FileMetadataDiff_Delete && !diff.GetFile().GetDir() {
			file, err := os.OpenFile(path, os.O_RDONLY, 0755)
			if err != nil {
				return err
			}
			log.Println("Uploading", path)
			err = client.UploadFile(ctx, path, file)
			if err != nil {
				return err
			}
			file.Close()
		}
	}
	log.Println("Uploading file list...")

	metadataBytes, err := proto.Marshal(fileMetadata)
	if err != nil {
		return err
	}
	err = client.UploadFile(ctx, ".jamsyncfilelist", bytes.NewReader(metadataBytes))
	if err != nil {
		return err
	}

	err = client.CommitChange()
	if err != nil {
		return err
	}
	log.Println("Committed changes.")

	return nil
}

func applyFileListDiff(fileMetadataDiff *pb.FileMetadataDiff, client *jam.Client) error {
	ctx := context.Background()
	for path, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && diff.GetFile().GetDir() {
			err := os.MkdirAll(path, os.ModePerm)
			if err != nil {
				return err
			}
		}
	}

	for path, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && !diff.GetFile().GetDir() {
			file, err := os.OpenFile(path+".jamtemp", os.O_RDWR|os.O_CREATE, 0755)
			if err != nil {
				log.Panic(err)
			}
			defer file.Close()

			fileContents, err := os.ReadFile(path)
			if err != nil {
				log.Panic(err)
			}

			err = client.DownloadFile(ctx, path, bytes.NewReader(fileContents), file)
			if err != nil {
				log.Panic(err)
			}

			newModTime := fileMetadataDiff.GetDiffs()[path].File.GetModTime().AsTime()
			err = os.Chtimes(path, newModTime, newModTime)
			if err != nil {
				log.Panic(err)
			}

			err = os.Rename(path+".jamtemp", path)
			if err != nil {
				log.Panic(err)
			}

			log.Println("Downloaded ", path)
		}
	}
	return nil
}

func currentDirectoryEmpty() (bool, error) {
	f, err := os.Open(".")
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func shouldExclude(path string) bool {
	return strings.HasPrefix(path, ".next") || strings.HasPrefix(path, "node_modules") || strings.HasSuffix(path, ".jamsync") || strings.HasPrefix(path, ".git") || strings.HasPrefix(path, "jb")
}
