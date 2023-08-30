package repo

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/pelletier/go-toml/v2"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	rootPathEnvVar = "GUARDIAN_PATH"

	cfgFileName = "guardian.toml"

	defaultRepoRoot = "~/.guardian"

	LogsDirName = "logs"

	NodeManagerContractAddr = "0x0000000000000000000000000000000000001001"
)

type Repo struct {
	Config *Config
}

// Exist check if the file with the given path exits.
func Exist(path string) bool {
	fi, err := os.Lstat(path)
	if fi != nil || (err != nil && !os.IsNotExist(err)) {
		return true
	}

	return false
}

func Load(repoRoot string) (*Repo, error) {
	rootPath, err := LoadRepoRootFromEnv(repoRoot)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig(rootPath)

	cfgPath := path.Join(repoRoot, cfgFileName)
	existConfig := Exist(cfgPath)
	if !existConfig {
		err := os.MkdirAll(rootPath, 0755)
		if err != nil {
			return nil, errors.Wrap(err, "failed to build default config")
		}

		if err := writeConfigWithEnv(cfgPath, cfg); err != nil {
			return nil, errors.Wrap(err, "failed to build default config")
		}
	} else {
		if err := CheckWritable(rootPath); err != nil {
			return nil, err
		}
		if err = readConfigFromFile(cfgPath, cfg); err != nil {
			return nil, err
		}
	}

	return &Repo{
		Config: cfg,
	}, nil
}

func (r *Repo) Flush() error {
	if err := writeConfigWithEnv(path.Join(r.Config.RepoRoot, cfgFileName), r.Config); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	return nil
}

func writeConfigWithEnv(cfgPath string, config any) error {
	if err := writeConfig(cfgPath, config); err != nil {
		return err
	}
	// write back environment variables first
	// TODO: wait viper support read from environment variables
	if err := readConfigFromFile(cfgPath, config); err != nil {
		return errors.Wrapf(err, "failed to read cfg from environment")
	}
	if err := writeConfig(cfgPath, config); err != nil {
		return err
	}
	return nil
}

func writeConfig(cfgPath string, config any) error {
	raw, err := MarshalConfig(config)
	if err != nil {
		return err
	}

	if err := os.WriteFile(cfgPath, []byte(raw), 0755); err != nil {
		return err
	}

	return nil
}

func MarshalConfig(config any) (string, error) {
	buf := bytes.NewBuffer([]byte{})
	e := toml.NewEncoder(buf)
	e.SetIndentTables(true)
	e.SetArraysMultiline(true)
	err := e.Encode(config)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func LoadRepoRootFromEnv(repoRoot string) (string, error) {
	if repoRoot != "" {
		return repoRoot, nil
	}
	repoRoot = os.Getenv(rootPathEnvVar)
	var err error
	if len(repoRoot) == 0 {
		repoRoot, err = homedir.Expand(defaultRepoRoot)
	}
	return repoRoot, err
}

func readConfigFromFile(cfgFilePath string, config any) error {
	vp := viper.New()
	vp.SetConfigFile(cfgFilePath)
	vp.SetConfigType("toml")
	return readConfig(vp, config)
}

func readConfig(vp *viper.Viper, config any) error {
	vp.AutomaticEnv()
	vp.SetEnvPrefix("GUARDIAN")
	replacer := strings.NewReplacer(".", "_")
	vp.SetEnvKeyReplacer(replacer)

	err := vp.ReadInConfig()
	if err != nil {
		return err
	}

	if err := vp.Unmarshal(config); err != nil {
		return err
	}

	return nil
}

func CheckWritable(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		// dir exists, make sure we can write to it
		testfile := filepath.Join(dir, "test")
		fi, err := os.Create(testfile)
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("%s is not writeable by the current user", dir)
			}
			return fmt.Errorf("unexpected error while checking writeablility of repo root: %s", err)
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// dir doesn't exist, check that we can create it
		return os.Mkdir(dir, 0775)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("cannot write to %s, incorrect permissions", err)
	}

	return err
}
