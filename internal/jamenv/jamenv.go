package jamenv

import (
	"os"
)

type JamEnv int

const (
	Prod JamEnv = iota
	Dev
	Local
)

func (e JamEnv) String() string {
	switch e {
	case Prod:
		return "prod"
	case Dev:
		return "dev"
	case Local:
		return "local"
	}
	return "unknown"
}

func Env() JamEnv {
	jamEnvString := os.Getenv("JAM_ENV")
	switch jamEnvString {
	case "dev":
		return Dev
	case "local":
		return Local
	default:
		return Prod
	}
}
