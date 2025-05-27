// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// precompiledTest defines the input/output pairs for precompiled contract tests.
type precompiledTest struct {
	Input, Expected string
	Gas             uint64
	Name            string
	NoBenchmark     bool // Benchmark primarily the worst-cases
}

// precompiledFailureTest defines the input/error pairs for precompiled
// contract failure tests.
type precompiledFailureTest struct {
	Input         string
	ExpectedError string
	Name          string
}

// allPrecompiles does not map to the actual set of precompiles, as it also contains
// repriced versions of precompiles at certain slots
var allPrecompiles = map[common.Address]PrecompiledContract{
	common.BytesToAddress([]byte{1}):    &ecrecover{},
	common.BytesToAddress([]byte{2}):    &sha256hash{},
	common.BytesToAddress([]byte{3}):    &ripemd160hash{},
	common.BytesToAddress([]byte{4}):    &dataCopy{},
	common.BytesToAddress([]byte{5}):    &bigModExp{eip2565: false},
	common.BytesToAddress([]byte{0xf5}): &bigModExp{eip2565: true},
	common.BytesToAddress([]byte{6}):    &bn256AddIstanbul{},
	common.BytesToAddress([]byte{7}):    &bn256ScalarMulIstanbul{},
	common.BytesToAddress([]byte{8}):    &bn256PairingIstanbul{},
	common.BytesToAddress([]byte{9}):    &blake2F{},
	common.BytesToAddress([]byte{0x0a}): &kzgPointEvaluation{},

	common.BytesToAddress([]byte{0x0f, 0x0a}): &bls12381G1Add{},
	common.BytesToAddress([]byte{0x0f, 0x0b}): &bls12381G1MultiExp{},
	common.BytesToAddress([]byte{0x0f, 0x0c}): &bls12381G2Add{},
	common.BytesToAddress([]byte{0x0f, 0x0d}): &bls12381G2MultiExp{},
	common.BytesToAddress([]byte{0x0f, 0x0e}): &bls12381Pairing{},
	common.BytesToAddress([]byte{0x0f, 0x0f}): &bls12381MapG1{},
	common.BytesToAddress([]byte{0x0f, 0x10}): &bls12381MapG2{},
	common.BytesToAddress([]byte{102}):        &blsSignatureVerify{},
	common.BytesToAddress([]byte{104}):        &verifyDoubleSignEvidence{},
}

// EIP-152 test vectors
var blake2FMalformedInputTests = []precompiledFailureTest{
	{
		Input:         "",
		ExpectedError: errBlake2FInvalidInputLength.Error(),
		Name:          "vector 0: empty input",
	},
	{
		Input:         "00000c48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5d182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b61626300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000001",
		ExpectedError: errBlake2FInvalidInputLength.Error(),
		Name:          "vector 1: less than 213 bytes input",
	},
	{
		Input:         "000000000c48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5d182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b61626300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000001",
		ExpectedError: errBlake2FInvalidInputLength.Error(),
		Name:          "vector 2: more than 213 bytes input",
	},
	{
		Input:         "0000000c48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5d182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b61626300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000002",
		ExpectedError: errBlake2FInvalidFinalFlag.Error(),
		Name:          "vector 3: malformed final block indicator flag",
	},
}

func testPrecompiled(addr string, test precompiledTest, t *testing.T) {
	p := allPrecompiles[common.HexToAddress(addr)]
	in := common.Hex2Bytes(test.Input)
	gas := p.RequiredGas(in)
	t.Run(fmt.Sprintf("%s-Gas=%d", test.Name, gas), func(t *testing.T) {
		if res, _, err := RunPrecompiledContract(p, in, gas, nil); err != nil {
			t.Error(err)
		} else if common.Bytes2Hex(res) != test.Expected {
			t.Errorf("Expected %v, got %v", test.Expected, common.Bytes2Hex(res))
		}
		if expGas := test.Gas; expGas != gas {
			t.Errorf("%v: gas wrong, expected %d, got %d", test.Name, expGas, gas)
		}
		// Verify that the precompile did not touch the input buffer
		exp := common.Hex2Bytes(test.Input)
		if !bytes.Equal(in, exp) {
			t.Errorf("Precompiled %v modified input data", addr)
		}
	})
}

func testPrecompiledOOG(addr string, test precompiledTest, t *testing.T) {
	p := allPrecompiles[common.HexToAddress(addr)]
	in := common.Hex2Bytes(test.Input)
	gas := p.RequiredGas(in) - 1

	t.Run(fmt.Sprintf("%s-Gas=%d", test.Name, gas), func(t *testing.T) {
		_, _, err := RunPrecompiledContract(p, in, gas, nil)
		if err.Error() != "out of gas" {
			t.Errorf("Expected error [out of gas], got [%v]", err)
		}
		// Verify that the precompile did not touch the input buffer
		exp := common.Hex2Bytes(test.Input)
		if !bytes.Equal(in, exp) {
			t.Errorf("Precompiled %v modified input data", addr)
		}
	})
}

func testPrecompiledFailure(addr string, test precompiledFailureTest, t *testing.T) {
	p := allPrecompiles[common.HexToAddress(addr)]
	in := common.Hex2Bytes(test.Input)
	gas := p.RequiredGas(in)
	t.Run(test.Name, func(t *testing.T) {
		_, _, err := RunPrecompiledContract(p, in, gas, nil)
		if err.Error() != test.ExpectedError {
			t.Errorf("Expected error [%v], got [%v]", test.ExpectedError, err)
		}
		// Verify that the precompile did not touch the input buffer
		exp := common.Hex2Bytes(test.Input)
		if !bytes.Equal(in, exp) {
			t.Errorf("Precompiled %v modified input data", addr)
		}
	})
}

func benchmarkPrecompiled(addr string, test precompiledTest, bench *testing.B) {
	if test.NoBenchmark {
		return
	}
	p := allPrecompiles[common.HexToAddress(addr)]
	in := common.Hex2Bytes(test.Input)
	reqGas := p.RequiredGas(in)

	var (
		res  []byte
		err  error
		data = make([]byte, len(in))
	)

	bench.Run(fmt.Sprintf("%s-Gas=%d", test.Name, reqGas), func(bench *testing.B) {
		bench.ReportAllocs()
		start := time.Now()
		bench.ResetTimer()
		for i := 0; i < bench.N; i++ {
			copy(data, in)
			res, _, err = RunPrecompiledContract(p, data, reqGas, nil)
		}
		bench.StopTimer()
		elapsed := uint64(time.Since(start))
		if elapsed < 1 {
			elapsed = 1
		}
		gasUsed := reqGas * uint64(bench.N)
		bench.ReportMetric(float64(reqGas), "gas/op")
		// Keep it as uint64, multiply 100 to get two digit float later
		mgasps := (100 * 1000 * gasUsed) / elapsed
		bench.ReportMetric(float64(mgasps)/100, "mgas/s")
		// Check if it is correct
		if err != nil {
			bench.Error(err)
			return
		}
		if common.Bytes2Hex(res) != test.Expected {
			bench.Errorf("Expected %v, got %v", test.Expected, common.Bytes2Hex(res))
			return
		}
	})
}

// Benchmarks the sample inputs from the ECRECOVER precompile.
func BenchmarkPrecompiledEcrecover(bench *testing.B) {
	t := precompiledTest{
		Input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Expected: "000000000000000000000000ceaccac640adf55b2028469bd36ba501f28b699d",
		Name:     "",
	}
	benchmarkPrecompiled("01", t, bench)
}

// Benchmarks the sample inputs from the SHA256 precompile.
func BenchmarkPrecompiledSha256(bench *testing.B) {
	t := precompiledTest{
		Input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Expected: "811c7003375852fabd0d362e40e68607a12bdabae61a7d068fe5fdd1dbbf2a5d",
		Name:     "128",
	}
	benchmarkPrecompiled("02", t, bench)
}

// Benchmarks the sample inputs from the RIPEMD precompile.
func BenchmarkPrecompiledRipeMD(bench *testing.B) {
	t := precompiledTest{
		Input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Expected: "0000000000000000000000009215b8d9882ff46f0dfde6684d78e831467f65e6",
		Name:     "128",
	}
	benchmarkPrecompiled("03", t, bench)
}

// Benchmarks the sample inputs from the identity precompile.
func BenchmarkPrecompiledIdentity(bench *testing.B) {
	t := precompiledTest{
		Input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Expected: "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Name:     "128",
	}
	benchmarkPrecompiled("04", t, bench)
}

// Benchmarks the sample inputs from the blsSignatureVerify precompile.
func BenchmarkPrecompiledBlsSignatureVerify(bench *testing.B) {
	t := precompiledTest{
		Input:    "f2d8e8e5bf354429e3ce8b97c4e88f7a0bf7bc917e856de762ed6d70dd8ec2d289a04d63285e4b45309e7c180ea82565e375dd62c7b80d957aea4c9b7e16bdb28a0f910036bd3220fe3d7614fb137a8f0a68b3c564ddd214b5041d8f7a124e6e7285ac42635e75eeb9051a052fb500b1c2bc23bd4290db59fc02be11f2b80896b6e22d5b8dd31ba2e49b13cd6be19fcd01c1e23af3e5165d88d8b9deaf38baa77770fa6a358e2eebdffd1bd8a1eb7386",
		Expected: "01",
		Name:     "",
	}
	benchmarkPrecompiled("66", t, bench)
}

// Tests the sample inputs from the ModExp EIP 198.
func TestPrecompiledModExp(t *testing.T)      { testJson("modexp", "05", t) }
func BenchmarkPrecompiledModExp(b *testing.B) { benchJson("modexp", "05", b) }

func TestPrecompiledModExpEip2565(t *testing.T)      { testJson("modexp_eip2565", "f5", t) }
func BenchmarkPrecompiledModExpEip2565(b *testing.B) { benchJson("modexp_eip2565", "f5", b) }

// Tests the sample inputs from the elliptic curve addition EIP 213.
func TestPrecompiledBn256Add(t *testing.T)      { testJson("bn256Add", "06", t) }
func BenchmarkPrecompiledBn256Add(b *testing.B) { benchJson("bn256Add", "06", b) }

// Tests OOG
func TestPrecompiledModExpOOG(t *testing.T) {
	modexpTests, err := loadJson("modexp")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range modexpTests {
		testPrecompiledOOG("05", test, t)
	}
}

// Tests the sample inputs from the elliptic curve scalar multiplication EIP 213.
func TestPrecompiledBn256ScalarMul(t *testing.T)      { testJson("bn256ScalarMul", "07", t) }
func BenchmarkPrecompiledBn256ScalarMul(b *testing.B) { benchJson("bn256ScalarMul", "07", b) }

// Tests the sample inputs from the elliptic curve pairing check EIP 197.
func TestPrecompiledBn256Pairing(t *testing.T)      { testJson("bn256Pairing", "08", t) }
func BenchmarkPrecompiledBn256Pairing(b *testing.B) { benchJson("bn256Pairing", "08", b) }

func TestPrecompiledBlake2F(t *testing.T)      { testJson("blake2F", "09", t) }
func BenchmarkPrecompiledBlake2F(b *testing.B) { benchJson("blake2F", "09", b) }

func TestPrecompileBlake2FMalformedInput(t *testing.T) {
	for _, test := range blake2FMalformedInputTests {
		testPrecompiledFailure("09", test, t)
	}
}

func TestPrecompiledEcrecover(t *testing.T) { testJson("ecRecover", "01", t) }

func testJson(name, addr string, t *testing.T) {
	tests, err := loadJson(name)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		testPrecompiled(addr, test, t)
	}
}

func testJsonFail(name, addr string, t *testing.T) {
	tests, err := loadJsonFail(name)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		testPrecompiledFailure(addr, test, t)
	}
}

func benchJson(name, addr string, b *testing.B) {
	tests, err := loadJson(name)
	if err != nil {
		b.Fatal(err)
	}
	for _, test := range tests {
		benchmarkPrecompiled(addr, test, b)
	}
}

func TestPrecompiledBLS12381G1Add(t *testing.T)      { testJson("blsG1Add", "f0a", t) }
func TestPrecompiledBLS12381G1Mul(t *testing.T)      { testJson("blsG1Mul", "f0b", t) }
func TestPrecompiledBLS12381G1MultiExp(t *testing.T) { testJson("blsG1MultiExp", "f0b", t) }
func TestPrecompiledBLS12381G2Add(t *testing.T)      { testJson("blsG2Add", "f0c", t) }
func TestPrecompiledBLS12381G2Mul(t *testing.T)      { testJson("blsG2Mul", "f0d", t) }
func TestPrecompiledBLS12381G2MultiExp(t *testing.T) { testJson("blsG2MultiExp", "f0d", t) }
func TestPrecompiledBLS12381Pairing(t *testing.T)    { testJson("blsPairing", "f0e", t) }
func TestPrecompiledBLS12381MapG1(t *testing.T)      { testJson("blsMapG1", "f0f", t) }
func TestPrecompiledBLS12381MapG2(t *testing.T)      { testJson("blsMapG2", "f10", t) }

func TestPrecompiledBlsSignatureVerify(t *testing.T) { testJson("blsSignatureVerify", "66", t) }

func TestPrecompiledPointEvaluation(t *testing.T) { testJson("pointEvaluation", "0a", t) }

func BenchmarkPrecompiledPointEvaluation(b *testing.B) { benchJson("pointEvaluation", "0a", b) }

func BenchmarkPrecompiledBLS12381G1Add(b *testing.B)      { benchJson("blsG1Add", "f0a", b) }
func BenchmarkPrecompiledBLS12381G1MultiExp(b *testing.B) { benchJson("blsG1MultiExp", "f0b", b) }
func BenchmarkPrecompiledBLS12381G2Add(b *testing.B)      { benchJson("blsG2Add", "f0c", b) }
func BenchmarkPrecompiledBLS12381G2MultiExp(b *testing.B) { benchJson("blsG2MultiExp", "f0d", b) }
func BenchmarkPrecompiledBLS12381Pairing(b *testing.B)    { benchJson("blsPairing", "f0e", b) }
func BenchmarkPrecompiledBLS12381MapG1(b *testing.B)      { benchJson("blsMapG1", "f0f", b) }
func BenchmarkPrecompiledBLS12381MapG2(b *testing.B)      { benchJson("blsMapG2", "f10", b) }

// Failure tests
func TestPrecompiledBLS12381G1AddFail(t *testing.T)      { testJsonFail("blsG1Add", "f0a", t) }
func TestPrecompiledBLS12381G1MulFail(t *testing.T)      { testJsonFail("blsG1Mul", "f0b", t) }
func TestPrecompiledBLS12381G1MultiExpFail(t *testing.T) { testJsonFail("blsG1MultiExp", "f0b", t) }
func TestPrecompiledBLS12381G2AddFail(t *testing.T)      { testJsonFail("blsG2Add", "f0c", t) }
func TestPrecompiledBLS12381G2MulFail(t *testing.T)      { testJsonFail("blsG2Mul", "f0d", t) }
func TestPrecompiledBLS12381G2MultiExpFail(t *testing.T) { testJsonFail("blsG2MultiExp", "f0d", t) }
func TestPrecompiledBLS12381PairingFail(t *testing.T)    { testJsonFail("blsPairing", "f0e", t) }
func TestPrecompiledBLS12381MapG1Fail(t *testing.T)      { testJsonFail("blsMapG1", "f0f", t) }
func TestPrecompiledBLS12381MapG2Fail(t *testing.T)      { testJsonFail("blsMapG2", "f10", t) }

func TestPrecompiledBlsSignatureVerifyFail(t *testing.T) { testJson("blsSignatureVerify", "66", t) }

func loadJson(name string) ([]precompiledTest, error) {
	data, err := os.ReadFile(fmt.Sprintf("testdata/precompiles/%v.json", name))
	if err != nil {
		return nil, err
	}
	var testcases []precompiledTest
	err = json.Unmarshal(data, &testcases)
	return testcases, err
}

func loadJsonFail(name string) ([]precompiledFailureTest, error) {
	data, err := os.ReadFile(fmt.Sprintf("testdata/precompiles/fail-%v.json", name))
	if err != nil {
		return nil, err
	}
	var testcases []precompiledFailureTest
	err = json.Unmarshal(data, &testcases)
	return testcases, err
}

// BenchmarkPrecompiledBLS12381G1MultiExpWorstCase benchmarks the worst case we could find that still fits a gaslimit of 10MGas.
func BenchmarkPrecompiledBLS12381G1MultiExpWorstCase(b *testing.B) {
	task := "0000000000000000000000000000000008d8c4a16fb9d8800cce987c0eadbb6b3b005c213d44ecb5adeed713bae79d606041406df26169c35df63cf972c94be1" +
		"0000000000000000000000000000000011bc8afe71676e6730702a46ef817060249cd06cd82e6981085012ff6d013aa4470ba3a2c71e13ef653e1e223d1ccfe9" +
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
	input := task
	for i := 0; i < 4787; i++ {
		input = input + task
	}
	testcase := precompiledTest{
		Input:       input,
		Expected:    "0000000000000000000000000000000005a6310ea6f2a598023ae48819afc292b4dfcb40aabad24a0c2cb6c19769465691859eeb2a764342a810c5038d700f18000000000000000000000000000000001268ac944437d15923dc0aec00daa9250252e43e4b35ec7a19d01f0d6cd27f6e139d80dae16ba1c79cc7f57055a93ff5",
		Name:        "WorstCaseG1",
		NoBenchmark: false,
	}
	benchmarkPrecompiled("f0c", testcase, b)
}

// BenchmarkPrecompiledBLS12381G2MultiExpWorstCase benchmarks the worst case we could find that still fits a gaslimit of 10MGas.
func BenchmarkPrecompiledBLS12381G2MultiExpWorstCase(b *testing.B) {
	task := "000000000000000000000000000000000d4f09acd5f362e0a516d4c13c5e2f504d9bd49fdfb6d8b7a7ab35a02c391c8112b03270d5d9eefe9b659dd27601d18f" +
		"000000000000000000000000000000000fd489cb75945f3b5ebb1c0e326d59602934c8f78fe9294a8877e7aeb95de5addde0cb7ab53674df8b2cfbb036b30b99" +
		"00000000000000000000000000000000055dbc4eca768714e098bbe9c71cf54b40f51c26e95808ee79225a87fb6fa1415178db47f02d856fea56a752d185f86b" +
		"000000000000000000000000000000001239b7640f416eb6e921fe47f7501d504fadc190d9cf4e89ae2b717276739a2f4ee9f637c35e23c480df029fd8d247c7" +
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
	input := task
	for i := 0; i < 1040; i++ {
		input = input + task
	}

	testcase := precompiledTest{
		Input:       input,
		Expected:    "0000000000000000000000000000000018f5ea0c8b086095cfe23f6bb1d90d45de929292006dba8cdedd6d3203af3c6bbfd592e93ecb2b2c81004961fdcbb46c00000000000000000000000000000000076873199175664f1b6493a43c02234f49dc66f077d3007823e0343ad92e30bd7dc209013435ca9f197aca44d88e9dac000000000000000000000000000000000e6f07f4b23b511eac1e2682a0fc224c15d80e122a3e222d00a41fab15eba645a700b9ae84f331ae4ed873678e2e6c9b000000000000000000000000000000000bcb4849e460612aaed79617255fd30c03f51cf03d2ed4163ca810c13e1954b1e8663157b957a601829bb272a4e6c7b8",
		Name:        "WorstCaseG2",
		NoBenchmark: false,
	}
	benchmarkPrecompiled("f0f", testcase, b)
}

func TestDoubleSignSlash(t *testing.T) {
	tc := precompiledTest{
		Input:    "f90597823039b902c7f902c4a056e5655f2d102c54a6fad8f44a1cf6948ad68eb8cb4f9d1130f73c446214f0daa01dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d4934794a889698683900857fa9ac54fbc972e88292b5387a08c1e2a3af9b5ab976f35b72d8524d247f8aae5e487486efa4f9ce813ae0a6420a08bc4953277d2197530d40187c3fd3181b56f7ca8322af2f06b3f08235249588ba070f1fd147f1ef508e1adde945f34cf65de46af4caab1d4853f10570a910ad14bb90100000000000000000010000000000000000000000000000000000000002000000000002000020000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000080000000000000080000000000000000000000000000000000000000000000000000008203e8820ba18401c9c38083013b448468348964b861d883010706846765746888676f312e32332e37856c696e757800000000000000629216dfb83abfddbf913703f8d67135585570ec7f04ee35645cb039db5bc975528a8e38024283e39ca33a711fb69d4495f5aa9efcb28f310c4717bbc3a0430c01a0000000000000000000000000000000000000000000000000000000000000000088000000000000000080a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b4218080a00000000000000000000000000000000000000000000000000000000000000000a0e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855b902c7f902c4a056e5655f2d102c54a6fad8f44a1cf6948ad68eb8cb4f9d1130f73c446214f0daa01dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d4934794a889698683900857fa9ac54fbc972e88292b5387a08c1e2a3af9b5ab976f35b72d8524d247f8aae5e487486efa4f9ce813ae0a6420a08bc4953277d2197530d40187c3fd3181b56f7ca8322af2f06b3f08235249588ba070f1fd147f1ef508e1adde945f34cf65de46af4caab1d4853f10570a910ad14bb90100000000000000000010000000000000000000000000000000000000002000000000002000020000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000080000000000000080000000000000000000000000000000000000000000000000000008203e8820ba18401c9c38083013b448468348964b8610001020306846765746888676f312e32332e37856c696e75780000000000000025f554ba7736a7659dc39ef3096ac952e3c80d0492d202a9eed4769ac64ea2b47a1ce56aab5ddd33b69c9520c5cced5fa22fbed7f8ddae6b3cdc20085542a49200a0000000000000000000000000000000000000000000000000000000000000000088000000000000000080a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b4218080a00000000000000000000000000000000000000000000000000000000000000000a0e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Expected: "a889698683900857fa9ac54fbc972e88292b53870000000000000000000000000000000000000000000000000000000000000ba1",
		Gas:      10000,
		Name:     "",
	}

	testPrecompiled("68", tc, t)
}

func TestDoubleSignSlashFailure(t *testing.T) {
	tc := precompiledFailureTest{
		Input:         "f9066b38b90332f9032fa01062d3d5015b9242bc193a9b0769f3d3780ecb55f97f40a752ae26d0b68cd0d8a0fae1a05fcb14bfd9b8a9f2b65007a9b6c2000de0627a73be644dd993d32342c494df87f0e2b8519ea2dd4abd8b639cdd628497ed25a0f385cc58ed297ff0d66eb5580b02853d3478ba418b1819ac659ee05df49b9794a0bf88464af369ed6b8cf02db00f0b9556ffa8d49cd491b00952a7f83431446638a00a6d0870e586a76278fbfdcedf76ef6679af18fc1f9137cfad495f434974ea81b901000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001a1010000000000000000000000000000000000000000000000000000000000000000830f4240830f42408465bc6996b90115d983010306846765746889676f312e32302e3131856c696e7578000053474aa9f8b25fb860b0844a5082bfaa2299d2a23f076e2f6b17b15f839cc3e7d5a875656f6733fd4b87ba3401f906d15f3dea263cd9a6076107c7db620a4630dd3832c4a4b57eb8f497e28a3d69e5c03b30205c4b45675747d513e1accd66329770f3c35b18c9d023f84c84023a5ad6a086a28d985d9a6c8e7f9a4feadd5ace0adba9818e1e1727edca755fcc0bd8344684023a5ad7a0bc3492196b2e68b8e6ceea87cfa7588b4d590089eb885c4f2c1e9d9fb450f7b980988e1b9d0beb91dab063e04879a24c43d33baae3759dee41fd62ffa83c77fd202bea27a829b49e8025bdd198393526dd12b223ab16052fd26a43f3aabf63e76901a0232c9ba2d41b40d36ed794c306747bcbc49bf61a0f37409c18bfe2b5bef26a2d880000000000000000b90332f9032fa01062d3d5015b9242bc193a9b0769f3d3780ecb55f97f40a752ae26d0b68cd0d8a0b2789a5357827ed838335283e15c4dcc42b9bebcbf2919a18613246787e2f96094df87f0e2b8519ea2dd4abd8b639cdd628497ed25a071ce4c09ee275206013f0063761bc19c93c13990582f918cc57333634c94ce89a00e095703e5c9b149f253fe89697230029e32484a410b4b1f2c61442d73c3095aa0d317ae19ede7c8a2d3ac9ef98735b049bcb7278d12f48c42b924538b60a25e12b901000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001a1010000000000000000000000000000000000000000000000000000000000000000830f4240830f42408465bc6996b90115d983010306846765746889676f312e32302e3131856c696e7578000053474aa9f8b25fb860b0844a5082bfaa2299d2a23f076e2f6b17b15f839cc3e7d5a875656f6733fd4b87ba3401f906d15f3dea263cd9a6076107c7db620a4630dd3832c4a4b57eb8f497e28a3d69e5c03b30205c4b45675747d513e1accd66329770f3c35b18c9d023f84c84023a5ad6a086a28d985d9a6c8e7f9a4feadd5ace0adba9818e1e1727edca755fcc0bd8344684023a5ad7a0bc3492196b2e68b8e6ceea87cfa7588b4d590089eb885c4f2c1e9d9fb450f7b9804c71ed015dd0c5c2d7393b68c2927f83f0a5da4c66f761f09e2f950cc610832c7876144599368404096ddef0eadacfde57717e2c7d23982b927285b797d41bfa00a0b56228685be711834d0f154292d07826dea42a0fad3e4f56c31470b7fbfbea26880000000000000000",
		ExpectedError: errInvalidEvidence.Error(),
		Name:          "",
	}

	testPrecompiledFailure("68", tc, t)
}
