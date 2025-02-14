// Copyright (c) 2022 Target Brands, Inc. All rights reserved.
//
// Use of this source code is governed by the LICENSE file in this repository.

package pipeline

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-vela/server/compiler"
	"github.com/go-vela/server/compiler/native"
	"github.com/go-vela/server/database"
	"github.com/go-vela/server/database/sqlite"
	"github.com/go-vela/server/router/middleware/org"
	"github.com/go-vela/server/router/middleware/repo"
	"github.com/go-vela/server/router/middleware/token"
	"github.com/go-vela/server/router/middleware/user"
	"github.com/go-vela/server/scm"
	"github.com/go-vela/server/scm/github"
	"github.com/go-vela/types"
	"github.com/go-vela/types/library"
	"github.com/urfave/cli/v2"
)

func TestPipeline_Retrieve(t *testing.T) {
	// setup types
	_pipeline := new(library.Pipeline)

	gin.SetMode(gin.TestMode)
	_context, _ := gin.CreateTestContext(nil)

	// setup tests
	tests := []struct {
		name    string
		context *gin.Context
		want    *library.Pipeline
	}{
		{
			name:    "context",
			context: _context,
			want:    _pipeline,
		},
	}

	// run tests
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ToContext(test.context, test.want)

			got := Retrieve(test.context)

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("Retrieve for %s is %v, want %v", test.name, got, test.want)
			}
		})
	}
}

func TestPipeline_Establish(t *testing.T) {
	// setup types
	r := new(library.Repo)
	r.SetID(1)
	r.SetUserID(1)
	r.SetHash("baz")
	r.SetOrg("foo")
	r.SetName("bar")
	r.SetFullName("foo/bar")
	r.SetVisibility("public")

	want := new(library.Pipeline)
	want.SetID(1)
	want.SetRepoID(1)
	want.SetCommit("48afb5bdc41ad69bf22588491333f7cf71135163")
	want.SetFlavor("")
	want.SetPlatform("")
	want.SetRef("refs/heads/master")
	want.SetType("yaml")
	want.SetVersion("1")
	want.SetExternalSecrets(false)
	want.SetInternalSecrets(false)
	want.SetServices(false)
	want.SetStages(false)
	want.SetSteps(false)
	want.SetTemplates(false)
	want.SetData([]byte{})

	got := new(library.Pipeline)

	// setup database
	db, _ := sqlite.NewTest()

	defer func() {
		db.Sqlite.Exec("delete from repos;")
		db.Sqlite.Exec("delete from pipelines;")
		_sql, _ := db.Sqlite.DB()
		_sql.Close()
	}()

	_ = db.CreateRepo(r)
	_ = db.CreatePipeline(want)

	// setup context
	gin.SetMode(gin.TestMode)

	resp := httptest.NewRecorder()
	context, engine := gin.CreateTestContext(resp)
	context.Request, _ = http.NewRequest(http.MethodGet, "/pipelines/foo/bar/48afb5bdc41ad69bf22588491333f7cf71135163", nil)

	// setup mock server
	engine.Use(func(c *gin.Context) { database.ToContext(c, db) })
	engine.Use(org.Establish())
	engine.Use(repo.Establish())
	engine.Use(Establish())
	engine.GET("/pipelines/:org/:repo/:pipeline", func(c *gin.Context) {
		got = Retrieve(c)

		c.Status(http.StatusOK)
	})

	// run test
	engine.ServeHTTP(context.Writer, context.Request)

	if resp.Code != http.StatusOK {
		t.Errorf("Establish returned %v, want %v", resp.Code, http.StatusOK)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Establish is %v, want %v", got, want)
	}
}

func TestPipeline_Establish_NoRepo(t *testing.T) {
	// setup database
	db, _ := sqlite.NewTest()
	defer func() { _sql, _ := db.Sqlite.DB(); _sql.Close() }()

	// setup context
	gin.SetMode(gin.TestMode)

	resp := httptest.NewRecorder()
	context, engine := gin.CreateTestContext(resp)
	context.Request, _ = http.NewRequest(http.MethodGet, "/pipelines/foo/bar/48afb5bdc41ad69bf22588491333f7cf71135163", nil)

	// setup mock server
	engine.Use(func(c *gin.Context) { database.ToContext(c, db) })
	engine.Use(Establish())

	// run test
	engine.ServeHTTP(context.Writer, context.Request)

	if resp.Code != http.StatusNotFound {
		t.Errorf("Establish returned %v, want %v", resp.Code, http.StatusNotFound)
	}
}

func TestPipeline_Establish_NoPipelineParameter(t *testing.T) {
	// setup types
	r := new(library.Repo)
	r.SetID(1)
	r.SetUserID(1)
	r.SetHash("baz")
	r.SetOrg("foo")
	r.SetName("bar")
	r.SetFullName("foo/bar")
	r.SetVisibility("public")

	// setup database
	db, _ := sqlite.NewTest()

	defer func() {
		db.Sqlite.Exec("delete from repos;")
		_sql, _ := db.Sqlite.DB()
		_sql.Close()
	}()

	_ = db.CreateRepo(r)

	// setup context
	gin.SetMode(gin.TestMode)

	resp := httptest.NewRecorder()
	context, engine := gin.CreateTestContext(resp)
	context.Request, _ = http.NewRequest(http.MethodGet, "/pipelines/foo/bar", nil)

	// setup mock server
	engine.Use(func(c *gin.Context) { database.ToContext(c, db) })
	engine.Use(org.Establish())
	engine.Use(repo.Establish())
	engine.Use(Establish())
	engine.GET("/pipelines/:org/:repo", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// run test
	engine.ServeHTTP(context.Writer, context.Request)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Establish returned %v, want %v", resp.Code, http.StatusBadRequest)
	}
}

func TestPipeline_Establish_NoPipeline(t *testing.T) {
	// setup types
	secret := "superSecret"

	r := new(library.Repo)
	r.SetID(1)
	r.SetUserID(1)
	r.SetHash("baz")
	r.SetOrg("foo")
	r.SetName("bar")
	r.SetFullName("foo/bar")
	r.SetVisibility("public")

	u := new(library.User)
	u.SetID(1)
	u.SetName("foo")
	u.SetToken("bar")
	u.SetHash("baz")
	u.SetAdmin(true)

	m := &types.Metadata{
		Database: &types.Database{
			Driver: "foo",
			Host:   "foo",
		},
		Queue: &types.Queue{
			Channel: "foo",
			Driver:  "foo",
			Host:    "foo",
		},
		Source: &types.Source{
			Driver: "foo",
			Host:   "foo",
		},
		Vela: &types.Vela{
			Address:    "foo",
			WebAddress: "foo",
		},
	}

	tok, err := token.CreateAccessToken(u, time.Minute*15)
	if err != nil {
		t.Errorf("unable to create access token: %v", err)
	}

	comp, err := native.New(cli.NewContext(nil, flag.NewFlagSet("test", 0), nil))
	if err != nil {
		t.Errorf("unable to create compiler: %v", err)
	}

	// setup database
	db, _ := sqlite.NewTest()

	defer func() {
		db.Sqlite.Exec("delete from repos;")
		db.Sqlite.Exec("delete from users;")
		_sql, _ := db.Sqlite.DB()
		_sql.Close()
	}()

	_ = db.CreateRepo(r)
	_ = db.CreateUser(u)

	// setup context
	gin.SetMode(gin.TestMode)

	resp := httptest.NewRecorder()
	context, engine := gin.CreateTestContext(resp)
	context.Request, _ = http.NewRequest(http.MethodGet, "/pipelines/foo/bar/148afb5bdc41ad69bf22588491333f7cf71135163", nil)
	context.Request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tok))

	// setup github mock server
	engine.GET("/api/v3/repos/:org/:repo/contents/:path", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusOK)
		c.File("testdata/yml.json")
	})

	s := httptest.NewServer(engine)
	defer s.Close()

	// setup client
	client, _ := github.NewTest(s.URL)

	// setup vela mock server
	engine.Use(func(c *gin.Context) { c.Set("metadata", m) })
	engine.Use(func(c *gin.Context) { c.Set("secret", secret) })
	engine.Use(func(c *gin.Context) { compiler.WithGinContext(c, comp) })
	engine.Use(func(c *gin.Context) { database.ToContext(c, db) })
	engine.Use(func(c *gin.Context) { scm.ToContext(c, client) })
	engine.Use(org.Establish())
	engine.Use(repo.Establish())
	engine.Use(user.Establish())
	engine.Use(Establish())
	engine.GET("/pipelines/:org/:repo/:pipeline", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// run test
	engine.ServeHTTP(context.Writer, context.Request)

	if resp.Code != http.StatusOK {
		t.Errorf("Establish returned %v, want %v", resp.Code, http.StatusOK)
	}
}
