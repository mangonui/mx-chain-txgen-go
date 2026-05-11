package scenarios

import (
	"math/big"
	"testing"
)

func TestEncodeData_NoArgs(t *testing.T) {
	if got := EncodeData("ESDTTransfer"); got != "ESDTTransfer" {
		t.Fatalf("got %q, want %q", got, "ESDTTransfer")
	}
}

func TestEncodeData_MultipleArgs(t *testing.T) {
	got := EncodeData("transfer", "deadbeef", "0a")
	want := "transfer@deadbeef@0a"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHexBytes_ZeroLengthIsEmpty(t *testing.T) {
	if HexBytes(nil) != "" {
		t.Fatalf("nil slice: want empty string")
	}
	if HexBytes([]byte{}) != "" {
		t.Fatalf("empty slice: want empty string")
	}
}

func TestHexBytes_RoundTripLowercase(t *testing.T) {
	if got := HexBytes([]byte{0xDE, 0xAD, 0xBE, 0xEF}); got != "deadbeef" {
		t.Fatalf("got %q, want %q", got, "deadbeef")
	}
}

func TestHexString_Utf8(t *testing.T) {
	// "WRK" is the default ticker the ESDT scenario emits at issuance time.
	if got := HexString("WRK"); got != "57524b" {
		t.Fatalf("got %q, want %q", got, "57524b")
	}
}

func TestHexBigInt_ZeroIsEmpty(t *testing.T) {
	if got := HexBigInt(big.NewInt(0)); got != "" {
		t.Fatalf("zero: got %q, want empty", got)
	}
	if got := HexBigInt(nil); got != "" {
		t.Fatalf("nil: got %q, want empty", got)
	}
}

func TestHexBigInt_OneIsTwoDigit(t *testing.T) {
	if got := HexBigInt(big.NewInt(1)); got != "01" {
		t.Fatalf("got %q, want %q", got, "01")
	}
}

func TestHexBigInt_StripsLeadingZeros(t *testing.T) {
	if got := HexBigInt(big.NewInt(256)); got != "0100" {
		t.Fatalf("got %q, want %q", got, "0100")
	}
}

func TestHexBigIntPadded_PadsLeft(t *testing.T) {
	if got := HexBigIntPadded(big.NewInt(1), 4); got != "00000001" {
		t.Fatalf("got %q, want %q", got, "00000001")
	}
}

func TestHexBigIntPadded_NilEmitsZeros(t *testing.T) {
	if got := HexBigIntPadded(nil, 2); got != "0000" {
		t.Fatalf("got %q, want %q", got, "0000")
	}
}

func TestHexUint64_OneByteValues(t *testing.T) {
	if got := HexUint64(0); got != "" {
		t.Fatalf("0: got %q, want empty", got)
	}
	if got := HexUint64(255); got != "ff" {
		t.Fatalf("255: got %q, want %q", got, "ff")
	}
}

func TestMustBigIntFromString_ParsesDecimal(t *testing.T) {
	v := MustBigIntFromString("1000000000000000000")
	want, _ := new(big.Int).SetString("1000000000000000000", 10)
	if v.Cmp(want) != 0 {
		t.Fatalf("got %s, want %s", v.String(), want.String())
	}
}

func TestMustBigIntFromString_PanicsOnGarbage(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on garbage input")
		}
	}()
	_ = MustBigIntFromString("not a number")
}
