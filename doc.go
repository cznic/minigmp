// Copyright 2017 The Minigmp Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package minigmp is a small implementation of a subset of GMP's mpn and mpz
// interfaces.
//
// Caveats
//
// - GPL license. May possibly become LGPL depending on the awaited response
// from licensing@fsf.org ticket #1225637.
//
// - The package currently supports only Linux/amd64. To attempt porting to
// Windows/amd64 try running
//
//	$ go generate
//
// on Windows/amd64.
//
// - ATM there are only a few simple tests covering just the basic arithmetic
// operations. The plan is to eventually run all translated-to-Go C tests in
// minigmp/tests during go generate. Linux/386 version does not yet pass even
// the simple tests, presumably because of a bug in the CCGO tool chain.
//
// - The automatically generated documentation is sparse and sometimes
// misplaced. Please consult the full documentation at
//
//	http://gmplib.org
package minigmp
