package facebook

import (
	"testing"
)

func TestPlainTextOnly(t *testing.T) {
	type args struct {
		text     string
		maxChars int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
		{
			name: "general",
			args: args{"❌ \rЗавершити !\t  ❎", 10},
			want: "Завершити",
		},
	}
	for _, tt := range tests {
		// for _, r := range tt.args.text {
		// 	t.Logf("unicode.IsPrint(%c) = %t", r, unicode.IsPrint(r))
		// 	t.Logf("unicode.IsSpace(%c) = %t", r, unicode.IsSpace(r))
		// 	t.Logf("unicode.IsPunct(%c) = %t", r, unicode.IsPunct(r))
		// 	t.Logf("unicode.IsDigit(%c) = %t", r, unicode.IsDigit(r))
		// 	t.Logf("unicode.IsNumber(%c) = %t", r, unicode.IsNumber(r))
		// 	t.Logf("unicode.IsLetter(%c) = %t", r, unicode.IsLetter(r))

		// 	t.Logf("unicode.IsMark(%c) = %t", r, unicode.IsMark(r))
		// 	t.Logf("unicode.IsSymbol(%c) = %t", r, unicode.IsSymbol(r))
		// 	t.Logf("unicode.IsControl(%c) = %t", r, unicode.IsControl(r))
		// 	t.Logf("unicode.IsGraphic(%c) = %t", r, unicode.IsGraphic(r))
		// 	t.Log("=============================")
		// }
		t.Run(tt.name, func(t *testing.T) {
			if got := scanTextPlain(tt.args.text, tt.args.maxChars); got != tt.want {
				t.Errorf("TextPlain() = %v, want %v", got, tt.want)
			} else {
				t.Logf("TextPlain() = %v,", got)
			}
		})
	}
}
