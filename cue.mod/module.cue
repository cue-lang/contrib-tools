module: "github.com/cue-lang/contrib-tools"
language: {
	version: "v0.10.0"
}
deps: {
	"cue.dev/x/githubactions@v0": {
		v:       "v0.1.0"
		default: true
	}
	"github.com/cue-lang/tmp/internal/ci@v0": {
		v:       "v0.0.1"
		default: true
	}
}
