// Copyright 2021 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package placement

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	pd "github.com/tikv/pd/client/http"
)

func TestNewFromYaml(t *testing.T) {
	_, err := NewConstraintsFromYaml([]byte("[]"))
	require.NoError(t, err)
	_, err = NewConstraintsFromYaml([]byte("]"))
	require.Error(t, err)
}

func TestNewConstraint(t *testing.T) {
	type TestCase struct {
		name  string
		input string
		label pd.LabelConstraint
		err   error
	}
	tests := []TestCase{
		{
			name:  "normal",
			input: "+zone=bj",
			label: pd.LabelConstraint{
				Key:    "zone",
				Op:     pd.In,
				Values: []string{"bj"},
			},
		},
		{
			name:  "normal with spaces",
			input: "-  dc  =  sh  ",
			label: pd.LabelConstraint{
				Key:    "dc",
				Op:     pd.NotIn,
				Values: []string{"sh"},
			},
		},
		{
			name:  "not tiflash",
			input: "-engine  =  tiflash  ",
			label: pd.LabelConstraint{
				Key:    "engine",
				Op:     pd.NotIn,
				Values: []string{"tiflash"},
			},
		},
		{
			name:  "not tiflash_compute",
			input: "-engine  =  tiflash_compute  ",
			label: pd.LabelConstraint{
				Key:    "engine",
				Op:     pd.NotIn,
				Values: []string{"tiflash_compute"},
			},
		},
		{
			name:  "disallow tiflash",
			input: "+engine=Tiflash",
			err:   ErrUnsupportedConstraint,
		},
		// invalid
		{
			name:  "invalid length",
			input: ",,,",
			err:   ErrInvalidConstraintFormat,
		},
		{
			name:  "invalid, lack = 1",
			input: "+    ",
			err:   ErrInvalidConstraintFormat,
		},
		{
			name:  "invalid, lack = 2",
			input: "+000",
			err:   ErrInvalidConstraintFormat,
		},
		{
			name:  "invalid op",
			input: "0000",
			err:   ErrInvalidConstraintFormat,
		},
		{
			name:  "empty key 1",
			input: "+ =zone1",
			err:   ErrInvalidConstraintFormat,
		},
		{
			name:  "empty key 2",
			input: "+  =   z",
			err:   ErrInvalidConstraintFormat,
		},
		{
			name:  "empty value 1",
			input: "+zone=",
			err:   ErrInvalidConstraintFormat,
		},
		{
			name:  "empty value 2",
			input: "+z  =   ",
			err:   ErrInvalidConstraintFormat,
		},
	}

	for _, test := range tests {
		label, err := NewConstraint(test.input)
		comment := fmt.Sprintf("%s: %v", test.name, err)
		if test.err == nil {
			require.NoError(t, err, comment)
			require.Equal(t, test.label, label, comment)
		} else {
			require.ErrorIs(t, err, test.err, comment)
		}
	}
}

func TestRestoreConstraint(t *testing.T) {
	type TestCase struct {
		name   string
		input  pd.LabelConstraint
		output string
		err    error
	}
	var tests []TestCase

	input, err := NewConstraint("+zone=bj")
	require.NoError(t, err)
	tests = append(tests, TestCase{
		name:   "normal, op in",
		input:  input,
		output: "+zone=bj",
	})

	input, err = NewConstraint("+  zone = bj  ")
	require.NoError(t, err)
	tests = append(tests, TestCase{
		name:   "normal with spaces, op in",
		input:  input,
		output: "+zone=bj",
	})

	input, err = NewConstraint("-  zone = bj  ")
	require.NoError(t, err)
	tests = append(tests, TestCase{
		name:   "normal with spaces, op not in",
		input:  input,
		output: "-zone=bj",
	})

	tests = append(tests, TestCase{
		name: "no values",
		input: pd.LabelConstraint{
			Op:     pd.In,
			Key:    "dc",
			Values: []string{},
		},
		err: ErrInvalidConstraintFormat,
	})

	tests = append(tests, TestCase{
		name: "multiple values",
		input: pd.LabelConstraint{
			Op:     pd.In,
			Key:    "dc",
			Values: []string{"dc1", "dc2"},
		},
		err: ErrInvalidConstraintFormat,
	})

	tests = append(tests, TestCase{
		name: "invalid op",
		input: pd.LabelConstraint{
			Op:     "[",
			Key:    "dc",
			Values: []string{},
		},
		err: ErrInvalidConstraintFormat,
	})

	for _, test := range tests {
		output, err := RestoreConstraint(&test.input)
		comment := fmt.Sprintf("%s: %v", test.name, err)
		if test.err == nil {
			require.NoError(t, err, comment)
			require.Equal(t, test.output, output, comment)
		} else {
			require.ErrorIs(t, err, test.err, comment)
		}
	}
}

func TestCompatibleWith(t *testing.T) {
	type TestCase struct {
		name   string
		i1     pd.LabelConstraint
		i2     pd.LabelConstraint
		output ConstraintCompatibility
	}
	var tests []TestCase

	i1, err := NewConstraint("+zone=sh")
	require.NoError(t, err)
	i2, err := NewConstraint("-zone=sh")
	require.NoError(t, err)
	tests = append(tests, TestCase{
		"case 2",
		i1, i2,
		ConstraintIncompatible,
	})

	i1, err = NewConstraint("+zone=bj")
	require.NoError(t, err)
	i2, err = NewConstraint("+zone=sh")
	require.NoError(t, err)
	tests = append(tests, TestCase{
		"case 3",
		i1, i2,
		ConstraintIncompatible,
	})

	i1, err = NewConstraint("+zone=sh")
	require.NoError(t, err)
	i2, err = NewConstraint("+zone=sh")
	require.NoError(t, err)
	tests = append(tests, TestCase{
		"case 1",
		i1, i2,
		ConstraintDuplicated,
	})

	i1, err = NewConstraint("+zone=sh")
	require.NoError(t, err)
	i2, err = NewConstraint("+dc=sh")
	require.NoError(t, err)
	tests = append(tests, TestCase{
		"normal 1",
		i1, i2,
		ConstraintCompatible,
	})

	i1, err = NewConstraint("-zone=sh")
	require.NoError(t, err)
	i2, err = NewConstraint("-zone=bj")
	require.NoError(t, err)
	tests = append(tests, TestCase{
		"normal 2",
		i1, i2,
		ConstraintCompatible,
	})

	for _, test := range tests {
		require.Equal(t, test.output, ConstraintCompatibleWith(&test.i1, &test.i2), test.name)
	}
}
