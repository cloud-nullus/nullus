// runtime-images 는 RuntimeImages() 를 한 줄에 하나씩 출력한다.
//
// 셸 스크립트가 목록을 따로 들면 그것도 드리프트 대상이 된다. 단일 출처를
// 그대로 읽게 한다.
package main

import (
	"fmt"

	"github.com/cloud-nullus/draft/internal/shared/domain"
)

func main() {
	for _, img := range domain.RuntimeImages() {
		fmt.Println(img)
	}
}
