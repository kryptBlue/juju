// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charm_test

import (
	"io/ioutil"
	gc "launchpad.net/gocheck"
	corecharm "launchpad.net/juju-core/charm"
	"launchpad.net/juju-core/testing"
	"launchpad.net/juju-core/worker/uniter/charm"
	"os"
	"path/filepath"
)

type DeployerSuite struct {
	testing.GitSuite
}

var _ = gc.Suite(&DeployerSuite{})

func (s *DeployerSuite) TestUnsetCharm(c *gc.C) {
	d := charm.NewDeployer(filepath.Join(c.MkDir(), "deployer"))
	err := d.Deploy(charm.NewGitDir(c.MkDir()))
	c.Assert(err, gc.ErrorMatches, "charm deployment failed: no charm set")
}

func (s *DeployerSuite) TestInstall(c *gc.C) {
	// Install.
	d := charm.NewDeployer(filepath.Join(c.MkDir(), "deployer"))
	bun := s.bundle(c, func(path string) {
		err := ioutil.WriteFile(filepath.Join(path, "some-file"), []byte("hello"), 0644)
		c.Assert(err, gc.IsNil)
	})
	err := d.Stage(bun, corecharm.MustParseURL("cs:s/c-1"))
	c.Assert(err, gc.IsNil)
	target := charm.NewGitDir(filepath.Join(c.MkDir(), "target"))
	err = d.Deploy(target)
	c.Assert(err, gc.IsNil)

	// Check content.
	data, err := ioutil.ReadFile(filepath.Join(target.Path(), "some-file"))
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, "hello")
	url, err := charm.ReadCharmURL(target)
	c.Assert(err, gc.IsNil)
	c.Assert(url, gc.DeepEquals, corecharm.MustParseURL("cs:s/c-1"))
	lines, err := target.Log()
	c.Assert(err, gc.IsNil)
	c.Assert(lines, gc.HasLen, 2)
	c.Assert(lines[0], gc.Matches, `[0-9a-f]{7} Deployed charm "cs:s/c-1".`)
	c.Assert(lines[1], gc.Matches, `[0-9a-f]{7} Imported charm "cs:s/c-1" from ".*".`)
}

func (s *DeployerSuite) TestUpgrade(c *gc.C) {
	// Install.
	d := charm.NewDeployer(filepath.Join(c.MkDir(), "deployer"))
	bun1 := s.bundle(c, func(path string) {
		err := ioutil.WriteFile(filepath.Join(path, "some-file"), []byte("hello"), 0644)
		c.Assert(err, gc.IsNil)
		err = os.Symlink("./some-file", filepath.Join(path, "a-symlink"))
		c.Assert(err, gc.IsNil)
	})
	err := d.Stage(bun1, corecharm.MustParseURL("cs:s/c-1"))
	c.Assert(err, gc.IsNil)
	target := charm.NewGitDir(filepath.Join(c.MkDir(), "target"))
	err = d.Deploy(target)
	c.Assert(err, gc.IsNil)

	// Upgrade.
	bun2 := s.bundle(c, func(path string) {
		err := ioutil.WriteFile(filepath.Join(path, "some-file"), []byte("goodbye"), 0644)
		c.Assert(err, gc.IsNil)
		err = ioutil.WriteFile(filepath.Join(path, "a-symlink"), []byte("not any more!"), 0644)
		c.Assert(err, gc.IsNil)
	})
	err = d.Stage(bun2, corecharm.MustParseURL("cs:s/c-2"))
	c.Assert(err, gc.IsNil)
	err = d.Deploy(target)
	c.Assert(err, gc.IsNil)

	// Check content.
	data, err := ioutil.ReadFile(filepath.Join(target.Path(), "some-file"))
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, "goodbye")
	data, err = ioutil.ReadFile(filepath.Join(target.Path(), "a-symlink"))
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, "not any more!")
	url, err := charm.ReadCharmURL(target)
	c.Assert(err, gc.IsNil)
	c.Assert(url, gc.DeepEquals, corecharm.MustParseURL("cs:s/c-2"))
	lines, err := target.Log()
	c.Assert(err, gc.IsNil)
	c.Assert(lines, gc.HasLen, 5)
	c.Assert(lines[0], gc.Matches, `[0-9a-f]{7} Upgraded charm to "cs:s/c-2".`)
}

func (s *DeployerSuite) TestConflict(c *gc.C) {
	// Install.
	d := charm.NewDeployer(filepath.Join(c.MkDir(), "deployer"))
	bun1 := s.bundle(c, func(path string) {
		err := ioutil.WriteFile(filepath.Join(path, "some-file"), []byte("hello"), 0644)
		c.Assert(err, gc.IsNil)
	})
	err := d.Stage(bun1, corecharm.MustParseURL("cs:s/c-1"))
	c.Assert(err, gc.IsNil)
	target := charm.NewGitDir(filepath.Join(c.MkDir(), "target"))
	err = d.Deploy(target)
	c.Assert(err, gc.IsNil)

	// Mess up target.
	err = ioutil.WriteFile(filepath.Join(target.Path(), "some-file"), []byte("mu!"), 0644)
	c.Assert(err, gc.IsNil)

	// Upgrade.
	bun2 := s.bundle(c, func(path string) {
		err := ioutil.WriteFile(filepath.Join(path, "some-file"), []byte("goodbye"), 0644)
		c.Assert(err, gc.IsNil)
	})
	err = d.Stage(bun2, corecharm.MustParseURL("cs:s/c-2"))
	c.Assert(err, gc.IsNil)
	err = d.Deploy(target)
	c.Assert(err, gc.Equals, charm.ErrConflict)

	// Check state.
	conflicted, err := target.Conflicted()
	c.Assert(err, gc.IsNil)
	c.Assert(conflicted, gc.Equals, true)

	// Revert and check initial content.
	err = target.Revert()
	c.Assert(err, gc.IsNil)
	data, err := ioutil.ReadFile(filepath.Join(target.Path(), "some-file"))
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, "mu!")
	conflicted, err = target.Conflicted()
	c.Assert(err, gc.IsNil)
	c.Assert(conflicted, gc.Equals, false)

	// Try to upgrade again.
	err = d.Deploy(target)
	c.Assert(err, gc.Equals, charm.ErrConflict)
	conflicted, err = target.Conflicted()
	c.Assert(err, gc.IsNil)
	c.Assert(conflicted, gc.Equals, true)

	// And again.
	err = d.Deploy(target)
	c.Assert(err, gc.Equals, charm.ErrConflict)
	conflicted, err = target.Conflicted()
	c.Assert(err, gc.IsNil)
	c.Assert(conflicted, gc.Equals, true)

	// Manually resolve, and commit.
	err = ioutil.WriteFile(filepath.Join(target.Path(), "some-file"), []byte("nu!"), 0644)
	c.Assert(err, gc.IsNil)
	err = target.Snapshotf("user resolved conflicts")
	c.Assert(err, gc.IsNil)
	conflicted, err = target.Conflicted()
	c.Assert(err, gc.IsNil)
	c.Assert(conflicted, gc.Equals, false)

	// Try a final upgrade to the same charm and check it doesn't write anything
	// except the upgrade log line.
	err = d.Deploy(target)
	c.Assert(err, gc.IsNil)
	data, err = ioutil.ReadFile(filepath.Join(target.Path(), "some-file"))
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, "nu!")
	conflicted, err = target.Conflicted()
	c.Assert(err, gc.IsNil)
	c.Assert(conflicted, gc.Equals, false)
	lines, err := target.Log()
	c.Assert(err, gc.IsNil)
	c.Assert(lines[0], gc.Matches, `[0-9a-f]{7} Upgraded charm to "cs:s/c-2".`)
}

func (s *DeployerSuite) bundle(c *gc.C, customize func(path string)) *corecharm.Bundle {
	base := c.MkDir()
	dirpath := testing.Charms.ClonedDirPath(base, "dummy")
	customize(dirpath)
	dir, err := corecharm.ReadDir(dirpath)
	c.Assert(err, gc.IsNil)
	bunpath := filepath.Join(base, "bundle")
	file, err := os.Create(bunpath)
	c.Assert(err, gc.IsNil)
	defer file.Close()
	err = dir.BundleTo(file)
	c.Assert(err, gc.IsNil)
	bun, err := corecharm.ReadBundle(bunpath)
	c.Assert(err, gc.IsNil)
	return bun
}
