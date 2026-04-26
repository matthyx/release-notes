package gh

import (
	"io"
	"os"
)

// stderr is the package-level log sink. Tests redirect it.
var stderr io.Writer = os.Stderr
