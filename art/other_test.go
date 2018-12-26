package art

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"strings"
	"testing"
	"time"
)

func Test_Env(t *testing.T) {
	name, _ := os.Hostname()
	OutTrace(name)
	current, _ := user.Current()
	OutTrace(current.Username)
}

func Test_Append(t *testing.T) {
	arr := []string{"0"}
	OutTrace("arr p=%p, l=%d, arr[-1]=%q", &arr, len(arr), arr[len(arr)-1])
	appval(arr)
	OutTrace("arr p=%p, l=%d, arr[-1]=%q", &arr, len(arr), arr[len(arr)-1])

	arr = []string{"0"}
	OutTrace("arr p=%p, l=%d, arr[-1]=%q", &arr, len(arr), arr[len(arr)-1])
	appptr(&arr)
	OutTrace("arr p=%p, l=%d, arr[-1]=%q", &arr, len(arr), arr[len(arr)-1])
}

func appval(arr []string) {
	for i := 0; i < 20; i++ {
		arr = append(arr, "val")
	}
}

func appptr(arr *[]string) {
	for i := 0; i < 20; i++ {
		*arr = append(*arr, "ptr")
	}
}

func Test_Point(t *testing.T) {

	arr := []string{"1", "2", "3"}
	OutTrace("----")
	OutTrace("arr    p=%p", &arr)
	OutTrace("arr[0] p=%p", &arr[0])
	OutTrace("arr[0] v=%q", arr[0])

	var infun = func(arr []string) {
		OutTrace("--infun--")
		OutTrace("arr    p=%p", &arr)
		OutTrace("arr[0] p=%p", &arr[0])
		OutTrace("arr[0] v=%q", arr[0])
		arr[0] = "infun"
	}

	prtval(arr)
	prtptr(&arr)
	infun(arr)

	OutTrace("----")
	OutTrace("arr    p=%p", &arr)
	OutTrace("arr[0] p=%p", &arr[0])
	OutTrace("arr[0] v=%q", arr[0])
}

func prtval(arr []string) {
	OutTrace("--prtval--")
	OutTrace("arr    p=%p", &arr)
	OutTrace("arr[0] p=%p", &arr[0])
	OutTrace("arr[0] v=%q", arr[0])
	arr[0] = "prtval"
}

func prtptr(arr *[]string) {
	OutTrace("--prtptr--")
	OutTrace("arr    p=%p", arr)
	OutTrace("arr[0] p=%p", &(*arr)[0])
	OutTrace("arr[0] v=%q", (*arr)[0])
	(*arr)[0] = "prtptr"
}

func Test_HttpPost(t *testing.T) {
	client := &http.Client{}
	url := ""
	for i := 0; i < 10; i++ {
		for a := 'a'; a <= 'z'; a++ {
			code := fmt.Sprintf("%d---%c", i, a)
			payload := strings.NewReader("reginvcode=" + code)
			req, _ := http.NewRequest("POST", url, payload)
			//设置header
			req.Header.Add("Connection", "keep-alive")
			req.Header.Add("Pragma", "no-cache")
			req.Header.Add("Cache-Control", "no-cache")

			res, _ := client.Do(req)
			body, _ := ioutil.ReadAll(res.Body)
			OutTrace("%s,%s", code, body)
			res.Body.Close()
			time.Sleep(3 * time.Second)
		}
	}
}
