// Code generated by "stringer -type Hardfork -linecomment ./pkg/config/hardfork.go"; DO NOT EDIT.

package config

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[HF2712FixSyscallFees-1]
}

const _Hardfork_name = "HF_2712_FixSyscallFees"

var _Hardfork_index = [...]uint8{0, 22}

func (i Hardfork) String() string {
	i -= 1
	if i >= Hardfork(len(_Hardfork_index)-1) {
		return "Hardfork(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _Hardfork_name[_Hardfork_index[i]:_Hardfork_index[i+1]]
}
