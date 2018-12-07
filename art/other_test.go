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
	fmt.Println(name)
	current, _ := user.Current()
	fmt.Println(current.Username)
}

func Test_Append(t *testing.T) {
	arr := []string{"0"}
	fmt.Printf("arr p=%p, l=%d, arr[-1]=%q\n", &arr, len(arr), arr[len(arr)-1])
	appval(arr)
	fmt.Printf("arr p=%p, l=%d, arr[-1]=%q\n", &arr, len(arr), arr[len(arr)-1])

	arr = []string{"0"}
	fmt.Printf("arr p=%p, l=%d, arr[-1]=%q\n", &arr, len(arr), arr[len(arr)-1])
	appptr(&arr)
	fmt.Printf("arr p=%p, l=%d, arr[-1]=%q\n", &arr, len(arr), arr[len(arr)-1])
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
	fmt.Println("----")
	fmt.Printf("arr    p=%p\n", &arr)
	fmt.Printf("arr[0] p=%p\n", &arr[0])
	fmt.Printf("arr[0] v=%q\n", arr[0])

	var infun = func(arr []string) {
		fmt.Println("--infun--")
		fmt.Printf("arr    p=%p\n", &arr)
		fmt.Printf("arr[0] p=%p\n", &arr[0])
		fmt.Printf("arr[0] v=%q\n", arr[0])
		arr[0] = "infun"
	}

	prtval(arr)
	prtptr(&arr)
	infun(arr)

	fmt.Println("----")
	fmt.Printf("arr    p=%p\n", &arr)
	fmt.Printf("arr[0] p=%p\n", &arr[0])
	fmt.Printf("arr[0] v=%q\n", arr[0])
}

func prtval(arr []string) {
	fmt.Println("--prtval--")
	fmt.Printf("arr    p=%p\n", &arr)
	fmt.Printf("arr[0] p=%p\n", &arr[0])
	fmt.Printf("arr[0] v=%q\n", arr[0])
	arr[0] = "prtval"
}

func prtptr(arr *[]string) {
	fmt.Println("--prtptr--")
	fmt.Printf("arr    p=%p\n", arr)
	fmt.Printf("arr[0] p=%p\n", &(*arr)[0])
	fmt.Printf("arr[0] v=%q\n", (*arr)[0])
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
			fmt.Printf("\n%s,%s", code, body)
			res.Body.Close()
			time.Sleep(3 * time.Second)
		}
	}
}
