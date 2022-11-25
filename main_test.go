package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq"
	"github.com/moby/moby/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	dsn string
	cid string
)

func TestMain(m *testing.M) {
	close, err := setup()
	if err != nil {
		log.Fatal(err)
	}
	defer close()

	code := m.Run()

	teardown()

	os.Exit(code)
}

func setup() (func() error, error) {
	cli, err := client.NewClientWithOpts()
	if err != nil {
		return nil, err
	}

	defer cli.Close()

	ctx := context.Background()

	_, err = cli.ImagePull(ctx, os.Getenv("POSTGRES_IMAGE"), types.ImagePullOptions{})

	if err != nil {
		return nil, err
	}

	cfg := &container.Config{
		Env: []string{
			`TZ=UTC`,
			`LANG=ja_JP.UTF-8`,
			`POSTGRES_DB=postgres `,
			`POSTGRES_USER=postgres`,
			`POSTGRES_PASSWORD=postgres`,
			`POSTGRES_INITDB_ARGS="--encoding=UTF-8"`,
			`POSTGRES_HOST_AUTH_METHOD=trust`,
		},
		Image:        os.Getenv("POSTGRES_IMAGE"),
		ExposedPorts: nat.PortSet{"5432/tcp": {}},
	}

	// search vacant port
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return l.Close, err
	}

	addr := l.Addr().String()
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return l.Close, err
	}

	dsn = fmt.Sprintf("postgres://postgres:postgres@localhost:%s/postgres?sslmode=disable", port)

	hcfg := &container.HostConfig{
		PortBindings: nat.PortMap{
			"5432/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: port,
				},
			},
		},
	}

	resp, err := cli.ContainerCreate(ctx, cfg, hcfg, nil, &v1.Platform{}, os.Getenv("APP_NAME"))
	if err != nil {
		return l.Close, err
	}

	cid = resp.ID

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return l.Close, err
	}

	time.Sleep(3 * time.Second)

	return l.Close, nil
}

func teardown() error {
	cli, err := client.NewClientWithOpts()
	if err != nil {
		return err
	}

	defer cli.Close()

	ctx := context.Background()

	timeout := time.Second
	if err := cli.ContainerStop(ctx, cid, &timeout); err != nil {
		return err
	}

	if err := cli.ContainerRemove(ctx, cid, types.ContainerRemoveOptions{}); err != nil {
		return err
	}

	return nil
}

func Test(t *testing.T) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf(err.Error())
	}

	rows, err := db.Query("SELECT 1")
	if err != nil {
		t.Fatalf(err.Error())
	}

	for rows.Next() {
		var i int
		rows.Scan(&i)
		if i != 1 {
			t.Errorf("rows.Scan() = %v, want 1", i)
		}
	}
}
