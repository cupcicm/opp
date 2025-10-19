//go:build !darwin

package core

import (
	"fmt"

	"github.com/atotto/clipboard"
)

func ClipboardWrite(pr *LocalPr, title string) {
	clipboard.WriteAll(fmt.Sprintf(":pr: <%s|%s>", title, pr.Url()))
}
