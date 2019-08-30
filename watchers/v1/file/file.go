package file

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/buger/jsonparser"

	"github.com/rapid7/cps/pkg/kv"
	"github.com/rapid7/cps/pkg/secret"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

var (
	// A global for the config struct. The config
	// struct below should just be exported (TODO).
	Config config
)

type config struct {
	directory string
	account   string
	region    string
}

// Polls every 60 seconds, causing the application
// to parse the files in the supplied directory.
func Poll(directory, account, region string) {
	Config = config{
		directory: directory,
		account:   account,
		region:    region,
	}

	Sync(time.Now())

	ticker := time.NewTicker(60 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				Sync(time.Now())
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

// This function performs the actual work of
// traversing the supplied directory and adding
// properties to the kv store.
func Sync(t time.Time) {
	absPath, _ := filepath.Abs(Config.directory)

	files, err := ioutil.ReadDir(absPath)
	if err != nil {
		log.Errorf("Error reading directory: %v", err)
	}

	for _, f := range files {
		properties := make(map[string]interface{})
		fn := f.Name()

		// Removes .json extension.
		shortPath := fn[0 : len(fn)-5]
		fullPath := absPath + "/" + fn
		path := Config.account + "/" + Config.region + "/" + shortPath

		j, _ := ioutil.ReadFile(fullPath)
		jsonparser.ObjectEach(j, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			switch dataTypeString := dataType.String(); dataTypeString {
			case "string":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				properties[string(key)] = string(value)
			case "number":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				var v interface{}
				if strings.Contains(string(value), ".") {
					v, _ = strconv.ParseFloat(string(value), 64)
				} else {
					v, _ = strconv.Atoi(string(value))
				}
				properties[string(key)] = v
			case "boolean":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				v, _ := strconv.ParseBool(string(value))
				properties[string(key)] = v
			case "null":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				properties[string(key)] = ""
			case "object":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				s, err := secret.GetSSMSecret(string(key), value)
				if err != nil {
					log.Error(err)
					return err
				}
				properties[string(key)] = s
			default:
				log.Errorf("Service: %v | Key: %v | Value %v | Type: %v | Unsupported! %v:%T", shortPath, string(key), string(value), dataTypeString, dataTypeString, dataTypeString)
			}

			return nil
		}, "properties")

		kv.WriteProperty(path, properties)
	}
}
