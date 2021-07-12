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

package strproc

import "testing"

func TestStr2int(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want int
	}{
		{
			name: "with letters",
			str:  "100G",
			want: 100,
		},
		{
			name: "with letters 2",
			str:  "m500",
			want: 500,
		},
		{
			name: "with letters 3",
			str:  "200Xz",
			want: 200,
		},
		{
			name: "with letters 4",
			str:  "3x21e1",
			want: 3211,
		},
		{
			name: "with symbol",
			str:  "0.5",
			want: 5,
		},
		{
			name: "with symbol 2",
			str:  "?12.5!-",
			want: 125,
		},
		{
			name: "with symbol and letters",
			str:  "a?1B3-4",
			want: 134,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Str2int(tt.str); got != tt.want {
				t.Errorf("Str2int() = %v, want %v", got, tt.want)
			}
		})
	}
}
