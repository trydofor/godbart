package internal

import (
	"fmt"
	"os"
	"os/user"
	"testing"
)

func TestEnv(t *testing.T) {
	name, _ := os.Hostname()
	fmt.Println(name)
	current, _ := user.Current()
	fmt.Println(current.Username)
}

func TestAppend(t *testing.T) {
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

func TestPoint(t *testing.T) {

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
