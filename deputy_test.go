// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package deputy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
)

type suite struct{}

var _ = gc.Suite(&suite{})

func Test(t *testing.T) {
	gc.TestingT(t)
}

type hasTimeout interface {
	IsTimeout() bool
}

func (*suite) TestRunTimeout(c *gc.C) {
	cmd := maker{
		timeout: time.Second * 2,
		c:       c,
	}.make()

	err := Deputy{Timeout: time.Millisecond * 100}.Run(cmd)

	c.Assert(err, gc.NotNil)
	if e, ok := err.(hasTimeout); !ok {
		c.Errorf("Error caused by timeout does not have Timeout function")
	} else {
		c.Assert(e.IsTimeout(), jc.IsTrue)
	}
}

func (*suite) TestRunNoTimeout(c *gc.C) {
	cmd := maker{
		c: c,
	}.make()

	err := Deputy{Timeout: time.Millisecond * 200}.Run(cmd)

	c.Assert(err, gc.IsNil)
}

func (*suite) TestStdoutErr(c *gc.C) {
	output := "foooo"
	cmd := maker{
		stdout: output,
		exit:   1,
		c:      c,
	}.make()
	err := Deputy{Errors: FromStdout}.Run(cmd)
	c.Assert(err, gc.ErrorMatches, ".*"+output)
}

func (*suite) TestStdoutOutput(c *gc.C) {
	output := "foooo"
	out := &bytes.Buffer{}
	cmd := maker{
		stdout: output,
		exit:   1,
		c:      c,
	}.make()
	cmd.Stdout = out
	err := Deputy{Errors: FromStdout}.Run(cmd)
	c.Assert(err, gc.ErrorMatches, ".*"+output)
	c.Assert(output, gc.Equals, strings.TrimSpace(out.String()))
}

func (*suite) TestStderrOutput(c *gc.C) {
	output := "foooo"
	out := &bytes.Buffer{}

	cmd := maker{
		stderr: output,
		exit:   1,
		c:      c,
	}.make()
	cmd.Stderr = out
	err := Deputy{Errors: FromStderr}.Run(cmd)
	c.Assert(err, gc.ErrorMatches, ".*"+output)
	c.Assert(output, gc.Equals, strings.TrimSpace(out.String()))
}

func (*suite) TestStderrErr(c *gc.C) {
	output := "foooo"

	cmd := maker{
		stderr: output,
		exit:   1,
		c:      c,
	}.make()
	err := Deputy{Errors: FromStderr}.Run(cmd)
	c.Assert(err, gc.ErrorMatches, ".*"+output)
}

func (*suite) TestLogs(c *gc.C) {
	stdout := "foo!\necho foo2!"
	stderr := "bar!\n>&2 echo bar2!"
	cmd := maker{
		stderr: stderr,
		stdout: stdout,
		c:      c,
	}.make()
	outs := []string{}
	errs := []string{}

	err := Deputy{
		StdoutLog: func(s string) { outs = append(outs, s) },
		StderrLog: func(s string) { errs = append(errs, s) },
	}.Run(cmd)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(outs, gc.DeepEquals, []string{"foo!", "foo2!"})
	c.Assert(errs, gc.DeepEquals, []string{"bar!", "bar2!"})
}

type maker struct {
	stdout  string
	stderr  string
	exit    int
	timeout time.Duration
	c       *gc.C
}

func (m maker) make() *exec.Cmd {
	if runtime.GOOS == "windows" {
		return m.winCmd()
	}
	return m.nixCmd()
}

func (m maker) winCmd() *exec.Cmd {
	var stderr string
	if len(m.stderr) > 0 {
		stderr = "echo " + m.stderr + " 1>&2\n"
	}
	var stdout string
	if len(m.stdout) > 0 {
		stdout = "echo " + m.stdout + "\n"
	}
	var data string
	if m.timeout > 0 {
		secs := int(math.Ceil(m.timeout.Seconds()))
		data = fmt.Sprintf("timeout /t %d\n%s%snexit %d", secs, stdout, stderr, m.exit)
	} else {
		data = fmt.Sprintf("%s%sexit %d", stdout, stderr, m.exit)
	}

	path := filepath.Join(m.c.MkDir(), "foo.bat")
	err := ioutil.WriteFile(path, []byte(data), 0744)
	m.c.Assert(err, jc.ErrorIsNil)
	return exec.Command(path)
}

func (m maker) nixCmd() *exec.Cmd {
	var stderr string
	if len(m.stderr) > 0 {
		stderr = ">&2 echo " + m.stderr + "\n"
	}
	var stdout string
	if len(m.stdout) > 0 {
		stdout = "echo " + m.stdout + "\n"
	}
	var data string
	if m.timeout > 0 {
		secs := int(math.Ceil(m.timeout.Seconds()))
		data = fmt.Sprintf("#!/bin/sh\nsleep %d\n%s%sexit %d", secs, stdout, stderr, m.exit)
	} else {
		data = fmt.Sprintf("#!/bin/sh\n%s%sexit %d", stdout, stderr, m.exit)
	}

	path := filepath.Join(m.c.MkDir(), "foo.sh")
	err := ioutil.WriteFile(path, []byte(data), 0744)
	m.c.Assert(err, jc.ErrorIsNil)
	return exec.Command(path)
}
