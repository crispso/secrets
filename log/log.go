// `log` is a singleton for conveniently handling debug output
package log

import (
	"jobbatical/secrets/options"
	"jobbatical/secrets/utils"
)

var PrintDebugln = utils.NoopDebugln

func init() {
	utils.ErrPrintln("noop %s", utils.NoopDebugln)
	utils.ErrPrintln("verbose %s", options.Verbose)
	utils.ErrPrintln("err %s", utils.ErrPrintln)
	utils.ErrPrintln("%s", PrintDebugln)
	if options.Verbose {
		PrintDebugln = utils.ErrPrintln
	}
	utils.ErrPrintln("%s", PrintDebugln)
}
