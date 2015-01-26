package hsup

import (
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"path/filepath"

	"github.com/fsouza/go-dockerclient"
)

type DockerDynoDriver struct {
	d *Docker
}

func (dd *DockerDynoDriver) Build(release *Release) error {
	if err := dd.connectDocker(); err != nil {
		return err
	}

	// TODO: map release.stack -> docker image name
	stack := "heroku/cedar:14"
	si, err := dd.d.StackStat(stack)
	if err != nil {
		return err
	}
	if si == nil {
		log.Fatalf("Stack not found = %v\n", stack)
	}

	imageName, err := dd.d.BuildSlugImage(si, release)
	if err != nil {
		log.Fatalln("could not build image:", err)
	}
	log.Println("Built image successfully")

	release.imageName = imageName
	return nil
}

func (dd *DockerDynoDriver) Start(ex *Executor) error {
	as := AppSerializable{
		Version: ex.Release.version,
		Env:     ex.Release.config,
		Stack:   ex.Release.stack,
		Processes: []FormationSerializable{
			{
				FArgs:     ex.Args,
				FQuantity: 1,
				FType:     ex.ProcessType,
			},
		},
	}

	// attach a timestamp as some extra entropy because container names must be
	// unique
	name := fmt.Sprintf("%v.%v", ex.Name(), time.Now().Unix())
	container, err := dd.d.c.CreateContainer(docker.CreateContainerOptions{
		Name: name,
		Config: &docker.Config{
			Cmd: []string{"setuidgid", "app",
				"/hsup", "-d", "abspath", "-a",
				ex.Release.appName, "--oneshot",
				"--start-number=" + ex.ProcessID,
				"start", ex.ProcessType},
			Env: []string{"HSUP_SKIP_BUILD=TRUE",
				"HSUP_CONTROL_GOB=" + as.ToBase64Gob()},
			Image:   ex.Release.imageName,
			Volumes: map[string]struct{}{"/hsup": {}},
		},
	})
	if err != nil {
		log.Fatalln("could not create container:", err)
	}
	ex.container = container

	where, err := filepath.Abs(linuxAmd64Path())
	if err != nil {
		return err
	}

	err = dd.d.c.StartContainer(ex.container.ID, &docker.HostConfig{
		Binds: []string{where + ":/hsup"},
	})
	if err != nil {
		log.Fatal(err)
	}

	go dd.d.c.Logs(docker.LogsOptions{
		Container:    container.ID,
		Stdout:       true,
		Stderr:       true,
		Follow:       true,
		OutputStream: os.Stdout,
	})

	return nil
}

func (dd *DockerDynoDriver) Wait(ex *Executor) (s *ExitStatus) {
	code, err := dd.d.c.WaitContainer(ex.container.ID)
	return &ExitStatus{Code: code, Err: err}
}

func (dd *DockerDynoDriver) Stop(ex *Executor) error {
	log.Println("Stopping container for", ex.Name())
	dd.d.c.KillContainer(docker.KillContainerOptions{
		ID:     ex.container.ID,
		Signal: docker.Signal(syscall.SIGTERM)})
	return dd.d.c.StopContainer(ex.container.ID, 10)
}

func (dd *DockerDynoDriver) connectDocker() error {
	if dd.d == nil {
		dd.d = &Docker{}
		if err := dd.d.Connect(); err != nil {
			dd.d = nil
			return err
		}
	}

	return nil
}
