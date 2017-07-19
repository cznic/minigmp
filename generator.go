// Copyright 2017 The Minigmp Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/scanner"
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/cznic/cc"
	"github.com/cznic/ccgo"
	"github.com/cznic/ccir"
	"github.com/cznic/internal/buffer"
	"github.com/cznic/strutil"
	"github.com/cznic/xc"
)

const (
	repo = "gmplib.org/gmp-6.1.2/mini-gmp/"

	defines = `
void *stderr;	
`

	prologue = `// Code generated by ccgo. DO NOT EDIT.

// To obtain the original C sources please visit gmplib.org.

// Header extracted from minigmp.c:
%s
package minigmp

import (
	"fmt"
	"math"
	"os"
	"path"
	"runtime"
	"unsafe"

	"github.com/cznic/crt"
)

func ftrace(s string, args ...interface{}) {
	_, fn, fl, _ := runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# %%s:%%d: %%v\n", path.Base(fn), fl, fmt.Sprintf(s, args...))
	os.Stderr.Sync()
}

`
)

var (
	cpp      = flag.Bool("cpp", false, "")
	errLimit = flag.Int("errlimit", 10, "")
	ndebug   = flag.Bool("ndebug", true, "")

	dict         = xc.Dict
	unconvertBin string
	lic          []byte
)

func goArch() string {
	if s := os.Getenv("GOARCH"); s != "" {
		return s
	}

	return runtime.GOARCH
}

func goOs() string {
	if s := os.Getenv("GOOS"); s != "" {
		return s
	}

	return runtime.GOOS
}

func findRepo(s string) string {
	s = filepath.FromSlash(s)
	for _, v := range strings.Split(strutil.Gopath(), string(os.PathListSeparator)) {
		p := filepath.Join(v, "src", s)
		fi, err := os.Lstat(p)
		if err != nil {
			continue
		}

		if fi.IsDir() {
			wd, err := os.Getwd()
			if err != nil {
				log.Fatal(err)
			}

			if p, err = filepath.Rel(wd, p); err != nil {
				log.Fatal(err)
			}

			return p
		}
	}
	return ""
}

func errStr(err error) string {
	switch x := err.(type) {
	case scanner.ErrorList:
		if len(x) != 1 {
			x.RemoveMultiples()
		}
		var b bytes.Buffer
		for i, v := range x {
			if i != 0 {
				b.WriteByte('\n')
			}
			b.WriteString(v.Error())
			if i == 9 {
				fmt.Fprintf(&b, "\n\t... and %v more errors", len(x)-10)
				break
			}
		}
		return b.String()
	default:
		return err.Error()
	}
}

func build(predef string, tus [][]string, ccgoOpts []ccgo.Option, opts ...cc.Opt) ([]*cc.TranslationUnit, []byte) {
	ndbg := ""
	if *ndebug {
		ndbg = "#define NDEBUG 1"
	}

	var lpos token.Position
	if *cpp {
		opts = append(opts, cc.Cpp(func(toks []xc.Token) {
			if len(toks) != 0 {
				p := toks[0].Position()
				if p.Filename != lpos.Filename {
					fmt.Fprintf(os.Stderr, "# %d %q\n", p.Line, p.Filename)
				}
				lpos = p
			}
			for _, v := range toks {
				os.Stderr.WriteString(cc.TokSrc(v))
			}
			os.Stderr.WriteString("\n")
		}))
	}

	var build []*cc.TranslationUnit
	for _, src := range tus {
		model, err := ccir.NewModel()
		if err != nil {
			log.Fatal(err)
		}

		ast, err := cc.Parse(
			fmt.Sprintf(`
%s
#define _CCGO 1
#define __arch__ %s
#define __os__ %s
#include <builtin.h>
%s
`, ndbg, goArch(), goOs(), predef),
			src,
			model,
			append([]cc.Opt{
				cc.AllowCompatibleTypedefRedefinitions(),
				cc.EnableAlignOf(),
				cc.EnableEmptyStructs(),
				cc.EnableNonConstStaticInitExpressions(),
				cc.ErrLimit(*errLimit),
				cc.KeepComments(),
				cc.SysIncludePaths([]string{ccir.LibcIncludePath}),
			}, opts...)...,
		)
		if err != nil {
			log.Fatal(errStr(err))
		}

		build = append(build, ast)
	}

	var out buffer.Bytes
	if err := ccgo.New(build, &out, ccgoOpts...); err != nil {
		log.Fatal(err)
	}

	return build, out.Bytes()
}

func macros(buf io.Writer, ast *cc.TranslationUnit) {
	fmt.Fprintf(buf, `const (
`)
	var a []string
	for k, v := range ast.Macros {
		if v.Value != nil && v.Type.Kind() != cc.Bool {
			switch fn := v.DefTok.Position().Filename; {
			case
				fn == "builtin.h",
				fn == "<predefine>",
				strings.HasPrefix(fn, "predefined_"):
				// ignore
			default:
				a = append(a, string(dict.S(k)))
			}
		}
	}
	sort.Strings(a)
	for _, v := range a {
		m := ast.Macros[dict.SID(v)]
		if m.Value == nil {
			log.Fatal("TODO")
		}

		switch t := m.Type; t.Kind() {
		case
			cc.Int, cc.UInt, cc.Long, cc.ULong, cc.LongLong, cc.ULongLong,
			cc.Float, cc.Double, cc.LongDouble, cc.Bool:
			fmt.Fprintf(buf, "X%s = %v\n", v, m.Value)
		case cc.Ptr:
			switch t := t.Element(); t.Kind() {
			case cc.Char:
				fmt.Fprintf(buf, "X%s = %q\n", v, dict.S(int(m.Value.(cc.StringLitID))))
			default:
				log.Fatalf("%v", t.Kind())
			}
		default:
			log.Fatalf("%v", t.Kind())
		}
	}

	a = a[:0]
	for _, v := range ast.Declarations.Identifiers {
		switch x := v.Node.(type) {
		case *cc.DirectDeclarator:
			d := x.TopDeclarator()
			id, _ := d.Identifier()
			if x.EnumVal == nil {
				break
			}

			a = append(a, string(dict.S(id)))
		default:
			log.Fatalf("%T", x)
		}
	}
	sort.Strings(a)
	for _, v := range a {
		dd := ast.Declarations.Identifiers[dict.SID(v)].Node.(*cc.DirectDeclarator)
		fmt.Fprintf(buf, "X%s = %v\n", v, dd.EnumVal)
	}
	fmt.Fprintf(buf, ")\n")
}

func header(f string) []byte {
	b, err := ioutil.ReadFile(f)
	if err != nil {
		log.Fatal(err)
	}

	var s scanner.Scanner
	s.Init(token.NewFileSet().AddFile(f, -1, len(b)), b, nil, scanner.ScanComments)
	var buf buffer.Bytes
	for {
		_, tok, lit := s.Scan()
		switch tok {
		case token.COMMENT:
			buf.WriteString(lit)
			buf.WriteByte('\n')
		default:
			return buf.Bytes()
		}
	}
}

func tidyComment(s string) string {
	switch {
	case strings.HasPrefix(s, "/*"):
		a := strings.Split("/"+s[1:len(s)-1], "\n")
		for i, v := range a {
			a[i] = "//  " + v
		}
		return strings.Join(a, "\n") + "/\n"
	case strings.HasPrefix(s, "//"):
		return "//  " + s[2:] + "\n"
	default:
		panic("internal error")
	}
}

func tidyComments(b []byte) string {
	var s scanner.Scanner
	s.Init(token.NewFileSet().AddFile("", -1, len(b)), b, nil, scanner.ScanComments)
	var a []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return strings.Replace(strings.Join(a, "\n"), "\n\n", "\n//\n", -1)
		}

		a = append(a, tidyComment(lit))
	}
}

func unconvert(pth string) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := os.Chdir(wd); err != nil {
			log.Fatal(err)
		}
	}()

	if err := os.Chdir(filepath.Dir(pth)); err != nil {
		log.Fatal(err)
	}

	if out, err := exec.Command(unconvertBin, "-apply").CombinedOutput(); err != nil {
		log.Fatalf("unconvert: %s\n%s", err, out)
	}
}

func lib() {
	const tweak = `
	crt\.Xfprintf\(tls, \(\*crt\.XFILE\)\(Xstderr\), str\([0-9]+\), unsafe\.Pointer\(unsafe\.Pointer\(_msg\)\)\)
	crt\.Xabort\(tls\)
`
	re := regexp.MustCompile(tweak)
	rp := findRepo(repo)
	if rp == "" {
		log.Fatalf("repository not found: %v", rp)
		return
	}

	ast, src := build(
		defines,
		[][]string{
			{filepath.Join(rp, "mini-gmp.h")},
		},
		[]ccgo.Option{
			ccgo.Library(),
		},
		cc.IncludePaths([]string{rp}),
	)
	_, src = build(
		defines,
		[][]string{
			{filepath.Join(rp, "mini-gmp.c")},
		},
		[]ccgo.Option{
			ccgo.LibcTypes(),
			ccgo.Library(),
		},
		cc.IncludePaths([]string{rp}),
	)

	var b bytes.Buffer
	fmt.Fprintf(&b, prologue, strings.TrimSpace(tidyComments(header(filepath.Join(rp, "mini-gmp.c")))))
	macros(&b, ast[0])
	b.Write(src)
	b2, err := format.Source(re.ReplaceAll(b.Bytes(), []byte(" panic(crt.GoString(_msg)) ")))
	if err != nil {
		b2 = b.Bytes()
	}
	tag := "minigmp"
	dst := fmt.Sprintf(filepath.Join(tag+"_%s_%s.go"), goOs(), goArch())
	if err := ioutil.WriteFile(dst, b2, 0664); err != nil {
		log.Fatal(err)
	}

	unconvert(dst)
}

func main() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
	var err error
	if unconvertBin, err = exec.LookPath("unconvert"); err != nil {
		log.Fatal("Please install the unconvert tool (go get -u github.com/mdempsky/unconvert)")
	}

	flag.Parse()
	lib()
}
