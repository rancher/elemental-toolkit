package plugins

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/utils"
	"github.com/pkg/errors"
	"github.com/zcalusic/sysinfo"
)

var system sysinfo.SysInfo

func init() {
	system.GetSysInfo()
}

type Console interface {
	Run(string, ...func(*exec.Cmd)) (string, error)
	Start(*exec.Cmd, ...func(*exec.Cmd)) error
	RunTemplate([]string, string) error
}

func templateSysData(l logger.Interface, s string) string {
	interpolateOpts := map[string]interface{}{}

	data, err := json.Marshal(&system)
	if err != nil {
		l.Warn(fmt.Sprintf("Failed marshalling '%s': %s", s, err.Error()))
		return s
	}
	l.Debug(string(data))

	err = json.Unmarshal(data, &interpolateOpts)
	if err != nil {
		l.Warn(fmt.Sprintf("Failed marshalling '%s': %s", s, err.Error()))
		return s
	}

	rendered, err := utils.TemplatedString(s, map[string]interface{}{"Values": interpolateOpts})
	if err != nil {
		l.Warn(fmt.Sprintf("Failed rendering '%s': %s", s, err.Error()))
		return s
	}
	return rendered
}

func download(url string) (string, error) {
	var resp *http.Response
	var err error
	for i := 0; i < 10; i++ {
		resp, err = http.Get(url)
		if err == nil || strings.Contains(err.Error(), "unsupported protocol scheme") {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return "", errors.Wrap(err, "failed while getting file")
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode/100 > 2 {
		return "", fmt.Errorf("%s %s", resp.Proto, resp.Status)
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	return string(bytes), err
}
