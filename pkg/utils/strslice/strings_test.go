/*
Copyright 2021 KubeCube Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package strslice

import (
	"reflect"
	"testing"
)

func TestContainsString(t *testing.T) {
	type args struct {
		slice []string
		s     string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "contain",
			args: args{slice: []string{"a", "b"}, s: "a"},
			want: true,
		},
		{
			name: "not contain",
			args: args{slice: []string{"a", "b", "c"}, s: "d"},
			want: false,
		},
		{
			name: "nil contain nothing",
			args: args{slice: nil, s: "a"},
			want: false,
		},
		{
			name: "empty contain nothing",
			args: args{slice: []string{}, s: "a"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsString(tt.args.slice, tt.args.s); got != tt.want {
				t.Errorf("ContainsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInsertString(t *testing.T) {
	type args struct {
		slice []string
		s     string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "dup",
			args: args{slice: []string{"a", "b"}, s: "a"},
			want: []string{"a", "b"},
		},
		{
			name: "not dup",
			args: args{slice: []string{"a", "b"}, s: "c"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "insert to nil",
			args: args{slice: nil, s: "a"},
			want: []string{"a"},
		},
		{
			name: "insert to empty",
			args: args{slice: []string{}, s: "a"},
			want: []string{"a"},
		},
		{
			name: "insert empty",
			args: args{slice: []string{"a", "b"}, s: ""},
			want: []string{"a", "b", ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InsertString(tt.args.slice, tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InsertString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveString(t *testing.T) {
	type args struct {
		slice []string
		s     string
	}
	tests := []struct {
		name       string
		args       args
		wantResult []string
	}{
		{
			name:       "remove exist",
			args:       args{slice: []string{"a", "b", "c"}, s: "a"},
			wantResult: []string{"b", "c"},
		},
		{
			name:       "remove not exist",
			args:       args{slice: []string{"a", "b"}, s: "c"},
			wantResult: []string{"a", "b"},
		},
		{
			name:       "remove empty",
			args:       args{slice: []string{"a", "b"}, s: ""},
			wantResult: []string{"a", "b"},
		},
		{
			name:       "remove from nil",
			args:       args{slice: nil, s: "a"},
			wantResult: []string{},
		},
		{
			name:       "remove from empty",
			args:       args{slice: []string{}, s: "a"},
			wantResult: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResult := RemoveString(tt.args.slice, tt.args.s); !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("RemoveString() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
