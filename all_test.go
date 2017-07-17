// Copyright 2017 The Minigmp Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package minigmp

import (
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/cznic/crt"
)

func caller(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(2)
	fmt.Fprintf(os.Stderr, "# caller: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	_, fn, fl, _ = runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# \tcallee: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

func dbg(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# dbg %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

func TODO(...interface{}) string { //TODOOK
	_, fn, fl, _ := runtime.Caller(1)
	return fmt.Sprintf("# TODO: %s:%d:\n", path.Base(fn), fl) //TODOOK
}

func use(...interface{}) {}

func init() {
	use(caller, dbg, TODO) //TODOOK
}

// ============================================================================

func sgn(b byte) byte {
	switch rand.Int31() % 2 {
	case 0:
		return b
	case 1:
		return '-'
	}
	panic("unreachable")
}

func test(t *testing.T, op string) {
	const (
		nTests  = 1000
		nDigits = 1000
	)

	tls := crt.NewTLS()

	defer tls.Close()

	ba := make([]byte, nDigits+1)
	bb := make([]byte, nDigits+1)
	for i := 0; i < nTests; i++ {
		ba = ba[:cap(ba)]
		bb = bb[:cap(ba)]
		for i := range ba {
			ba[i] = byte('0' + rand.Intn(10))
			bb[i] = byte('0' + rand.Intn(10))
		}
		ok := false
		for _, v := range bb[1:] {
			if v != '0' {
				ok = true
			}
		}
		if !ok {
			bb[1] = '1'
		}
		ba[0] = sgn(ba[0])
		bb[0] = sgn(bb[0])
		ba = ba[:rand.Intn(nDigits-1)+3]
		bb = bb[:rand.Intn(nDigits-1)+3]
		func() {
			var r, x, y [1]Xmpz_srcptr
			Xmpz_init(tls, &r)
			Xmpz_init(tls, &x)
			Xmpz_init(tls, &y)
			sa := string(ba)
			sb := string(bb)
			ca := crt.CString(sa)
			cb := crt.CString(sb)

			defer func() {
				Xmpz_clear(tls, &r)
				Xmpz_clear(tls, &x)
				Xmpz_clear(tls, &y)
				crt.Free(ca)
				crt.Free(cb)
			}()

			Xmpz_set_str(tls, &x, (*int8)(ca), 10)
			Xmpz_set_str(tls, &y, (*int8)(cb), 10)
			switch op {
			case "+":
				Xmpz_add(tls, &r, &x, &y)
			case "-":
				Xmpz_sub(tls, &r, &x, &y)
			case "*":
				Xmpz_mul(tls, &r, &x, &y)
			case "/":
				Xmpz_tdiv_q(tls, &r, &x, &y)
			case "%":
				Xmpz_tdiv_r(tls, &r, &x, &y)
			default:
				t.Fatal(op)
			}

			cr := Xmpz_get_str(tls, nil, 10, &r)

			defer crt.Free(unsafe.Pointer(cr))

			bigX, _ := big.NewInt(0).SetString(sa, 10)
			bigY, _ := big.NewInt(0).SetString(sb, 10)
			switch op {
			case "+":
				bigX.Add(bigX, bigY)
			case "-":
				bigX.Sub(bigX, bigY)
			case "*":
				bigX.Mul(bigX, bigY)
			case "/":
				bigX.Quo(bigX, bigY)
			case "%":
				bigX.Rem(bigX, bigY)
			default:
				t.Fatal(op)
			}

			if g, e := crt.GoString(cr), bigX.String(); g != e {
				t.Fatalf("%v %s %s = %v, got %v", sa, op, sb, e, g)
			}
		}()
	}
}

func TestAdd(t *testing.T) { test(t, "+") }
func TestSub(t *testing.T) { test(t, "-") }
func TestMul(t *testing.T) { test(t, "*") }
func TestDiv(t *testing.T) { test(t, "/") }
func TestRem(t *testing.T) { test(t, "%") }
