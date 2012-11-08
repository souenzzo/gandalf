package repository

import (
	"github.com/globocom/commandmocker"
	"github.com/globocom/config"
	"github.com/globocom/gandalf/db"
	"github.com/globocom/tsuru/fs"
	fstesting "github.com/globocom/tsuru/fs/testing"
	"labix.org/v2/mgo/bson"
	. "launchpad.net/gocheck"
	"path"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type S struct {
	tmpdir string
}

var _ = Suite(&S{})

func (s *S) SetUpSuite(c *C) {
	err := config.ReadConfigFile("../etc/gandalf.conf")
	c.Assert(err, IsNil)
	c.Check(err, IsNil)
}

func (s *S) TestNewShouldCreateANewRepository(c *C) {
	tmpdir, err := commandmocker.Add("git", "$*")
	c.Assert(err, IsNil)
	defer commandmocker.Remove(tmpdir)
	users := []string{"smeagol", "saruman"}
	r, err := New("myRepo", users, true)
	c.Assert(err, IsNil)
	defer db.Session.Repository().Remove(bson.M{"_id": "myRepo"})
	c.Assert(r.Name, Equals, "myRepo")
	c.Assert(r.Users, DeepEquals, users)
	c.Assert(r.IsPublic, Equals, true)
}

func (s *S) TestNewShouldRecordItOnDatabase(c *C) {
	tmpdir, err := commandmocker.Add("git", "$*")
	c.Assert(err, IsNil)
	defer commandmocker.Remove(tmpdir)
	r, err := New("someRepo", []string{"smeagol"}, true)
	defer db.Session.Repository().Remove(bson.M{"_id": "someRepo"})
	c.Assert(err, IsNil)
	err = db.Session.Repository().Find(bson.M{"_id": "someRepo"}).One(&r)
	c.Assert(err, IsNil)
	c.Assert(r.Name, Equals, "someRepo")
	c.Assert(r.Users, DeepEquals, []string{"smeagol"})
	c.Assert(r.IsPublic, Equals, true)
}

func (s *S) TestNewBreaksOnValidationError(c *C) {
	_, err := New("", []string{"smeagol"}, false)
	c.Check(err, NotNil)
	expected := "Validation Error: repository name is not valid"
	got := err.Error()
	c.Assert(got, Equals, expected)
}

func (s *S) TestRepositoryIsNotValidWithoutAName(c *C) {
	r := Repository{Users: []string{"gollum"}, IsPublic: true}
	v, err := r.isValid()
	c.Assert(v, Equals, false)
	c.Check(err, NotNil)
	got := err.Error()
	expected := "Validation Error: repository name is not valid"
	c.Assert(got, Equals, expected)
}

func (s *S) TestRepositoryIsNotValidWithInvalidName(c *C) {
	r := Repository{Name: "foo bar", Users: []string{"gollum"}, IsPublic: true}
	v, err := r.isValid()
	c.Assert(v, Equals, false)
	c.Check(err, NotNil)
	got := err.Error()
	expected := "Validation Error: repository name is not valid"
	c.Assert(got, Equals, expected)
}

func (s *S) TestRepositoryShoudBeInvalidWIthoutAnyUsers(c *C) {
	r := Repository{Name: "foo_bar", Users: []string{}, IsPublic: true}
	v, err := r.isValid()
	c.Assert(v, Equals, false)
	c.Assert(err, NotNil)
	got := err.Error()
	expected := "Validation Error: repository should have at least one user"
	c.Assert(got, Equals, expected)
}

func (s *S) TestRepositoryShouldBeValidWithoutIsPublic(c *C) {
	r := Repository{Name: "someName", Users: []string{"smeagol"}}
	v, _ := r.isValid()
	c.Assert(v, Equals, true)
}

func (s *S) TestNewShouldCreateNewGitBareRepository(c *C) {
	tmpdir, err := commandmocker.Add("git", "$*")
	c.Assert(err, IsNil)
	defer commandmocker.Remove(tmpdir)
	_, err = New("myRepo", []string{"pumpkin"}, true)
	c.Assert(err, IsNil)
	defer db.Session.Repository().Remove(bson.M{"_id": "myRepo"})
	c.Assert(commandmocker.Ran(tmpdir), Equals, true)
}

func (s *S) TestNewShouldNotStoreRepoInDbWhenBareCreationFails(c *C) {
	dir, err := commandmocker.Error("git", "", 1)
	c.Check(err, IsNil)
	defer commandmocker.Remove(dir)
	r, err := New("myRepo", []string{"pumpkin"}, true)
	c.Check(err, NotNil)
	err = db.Session.Repository().Find(bson.M{"_id": r.Name}).One(&r)
	c.Assert(err, ErrorMatches, "^not found$")
}

func (s *S) TestRemoveShouldRemoveBareRepositoryFromFileSystem(c *C) {
	tmpdir, err := commandmocker.Add("git", "$*")
	c.Assert(err, IsNil)
	defer commandmocker.Remove(tmpdir)
	rfs := &fstesting.RecordingFs{FileContent: "foo"}
	fsystem = rfs
	defer func() { fsystem = nil }()
	r, err := New("myRepo", []string{"pumpkin"}, false)
	c.Assert(err, IsNil)
	err = Remove(r)
	c.Assert(err, IsNil)
	action := "removeall " + path.Join(bareLocation(), "myRepo.git")
	c.Assert(rfs.HasAction(action), Equals, true)
}

func (s *S) TestRemoveShouldRemoveRepositoryFromDatabase(c *C) {
	tmpdir, err := commandmocker.Add("git", "$*")
	c.Assert(err, IsNil)
	defer commandmocker.Remove(tmpdir)
	rfs := &fstesting.RecordingFs{FileContent: "foo"}
	fsystem = rfs
	defer func() { fsystem = nil }()
	r, err := New("myRepo", []string{"pumpkin"}, false)
	c.Assert(err, IsNil)
	err = Remove(r)
	c.Assert(err, IsNil)
	err = db.Session.Repository().Find(bson.M{"_id": r.Name}).One(&r)
	c.Assert(err, ErrorMatches, "^not found$")
}

func (s *S) TestRemoveShouldReturnMeaningfulErrorWhenRepositoryDoesNotExistsInDatabase(c *C) {
	rfs := &fstesting.RecordingFs{FileContent: "foo"}
	fsystem = rfs
	defer func() { fsystem = nil }()
	r := &Repository{Name: "fooBar"}
	err := Remove(r)
	c.Assert(err, ErrorMatches, "^Could not remove repository: not found$")
}

func (s *S) TestRemoteShouldFormatAndReturnTheGitRemote(c *C) {
	r := Repository{Name: "lol"}
	remote := r.Remote()
	c.Assert(remote, Equals, "git@gandalfhost.com:lol.git")
}

func (s *S) TestFsystemShouldSetGlobalFsystemWhenItsNil(c *C) {
	fsystem = nil
	fsys := filesystem()
	_, ok := fsys.(fs.Fs)
	c.Assert(ok, Equals, true)
}
