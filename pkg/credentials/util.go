package credentials

import (
	"path/filepath"
)

type CredentialHelperDirs struct {
	RevisionFile, BinDir, RepoDir string
}

func GetCredentialHelperDirs(cacheDir string) CredentialHelperDirs {
	return CredentialHelperDirs{
		RevisionFile: filepath.Join(cacheDir, "repos", "gptscript-credential-helpers", "revision"),
		BinDir:       filepath.Join(cacheDir, "repos", "gptscript-credential-helpers", "bin"),
		RepoDir:      filepath.Join(cacheDir, "repos", "gptscript-credential-helpers", "repo"),
	}
}
