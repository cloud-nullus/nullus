// Nullus 로그인 테마가 차트를 타고 Keycloak 파드까지 실제로 들어가는지 본다.
//
// 이 배선이 조용히 끊기는 방식은 하나가 아니다 — ConfigMap 에 파일이 빠지거나,
// 마운트 경로가 어긋나거나, realm 이 가리키는 테마 이름이 폴더 이름과 달라지면
// Keycloak 은 오류 없이 기본 화면으로 되돌아간다. 설치는 초록색인데 로그인
// 화면만 예전으로 돌아가 있는 상태라 눈으로 보기 전엔 모른다.
package helm_test

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// 테마 폴더 이름. realm 의 loginTheme 이 이 이름을 가리킨다.
const themeName = "nullus"

// 컨테이너 안에서 테마가 놓여야 하는 자리. 이미지 계열마다 다르다 — bitnami 는
// /opt/bitnami/keycloak, 공식 quay.io 이미지는 /opt/keycloak 이다.
//
// 이 값을 상수로 박아 두면 마운트 경로와 서로만 맞고 **실제 이미지와는 어긋난
// 채로** 초록불이 난다. 실제로 그렇게 됐다 — 차트는 bitnamilegacy/keycloak 을
// 띄우는데 /opt/keycloak 에 얹어 두어, 파일이 Keycloak 이 보지 않는 자리에
// 놓였고 로그인 화면은 조용히 기본 테마로 돌아갔다. 그래서 values 의 이미지에서
// 끌어낸다.
func themeRootFor(t *testing.T) string {
	t.Helper()
	var v struct {
		Keycloak struct {
			Image struct {
				Repository string `yaml:"repository"`
			} `yaml:"image"`
		} `yaml:"keycloak"`
	}
	raw, err := os.ReadFile(filepath.Join(chartDir(t), "values.yaml"))
	if err != nil {
		t.Fatalf("values.yaml 읽기: %v", err)
	}
	if err := yaml.Unmarshal(raw, &v); err != nil {
		t.Fatalf("values.yaml 파싱: %v", err)
	}
	repo := v.Keycloak.Image.Repository
	if repo == "" {
		t.Fatal("keycloak.image.repository 가 비었다 — 테마 경로를 정할 수 없다")
	}
	base := "/opt/keycloak"
	if strings.HasPrefix(repo, "bitnami") {
		base = "/opt/bitnami/keycloak"
	}
	return base + "/themes/" + themeName + "/login"
}

func chartDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("nullus")
	if err != nil {
		t.Fatalf("차트 경로: %v", err)
	}
	return dir
}

// renderChart 는 helm template 결과를 돌려준다. helm 이 없으면 건너뛴다.
func renderChart(t *testing.T, extraArgs ...string) string {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm 이 PATH 에 없다 — 차트 렌더 검증을 건너뛴다")
	}
	args := append([]string{"template", "nullus", chartDir(t)}, extraArgs...)
	out, err := exec.Command("helm", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("helm %s 실패: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// themeFile 은 테마 파일 하나다. dir 은 login/ 아래 상대 폴더("" 면 login 바로 밑).
type themeFile struct {
	name string
	dir  string
	path string
}

// themeFiles 는 차트가 싣고 가야 할 테마 파일을 디스크에서 읽는다. 목록을 손으로
// 적으면 파일이 늘었을 때 테스트가 같이 늙는다.
func themeFiles(t *testing.T) []themeFile {
	t.Helper()
	base := filepath.Join(chartDir(t), "files/keycloak-theme", themeName, "login")
	var files []themeFile
	seen := map[string]string{}
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return relErr
		}
		name := filepath.Base(path)
		if prev, dup := seen[name]; dup {
			// ConfigMap 키는 파일 이름 하나뿐이라 같은 이름이 둘이면 하나가
			// 조용히 사라진다.
			t.Errorf("파일 이름이 겹친다: %s 와 %s", prev, rel)
		}
		seen[name] = rel
		files = append(files, themeFile{name: name, dir: filepath.Dir(rel), path: path})
		return nil
	})
	if err != nil {
		t.Fatalf("테마 폴더를 읽지 못했다 (%s): %v", base, err)
	}
	if len(files) == 0 {
		t.Fatalf("테마 파일이 하나도 없다: %s", base)
	}
	return files
}

// k8sObject 는 렌더 결과에서 우리가 보는 만큼만 뜬다.
// cmSource 는 볼륨이 참조하는 ConfigMap 하나다. items 를 주면 키를 원하는
// 경로에 놓을 수 있다 — ConfigMap 키에는 "/" 를 못 넣지만 이 path 에는 넣는다.
type cmSource struct {
	Name  string `yaml:"name"`
	Items []struct {
		Key  string `yaml:"key"`
		Path string `yaml:"path"`
	} `yaml:"items"`
}

type volume struct {
	Name      string    `yaml:"name"`
	ConfigMap *cmSource `yaml:"configMap"`
	Projected *struct {
		Sources []struct {
			ConfigMap *cmSource `yaml:"configMap"`
		} `yaml:"sources"`
	} `yaml:"projected"`
}

// sources 는 볼륨이 실제로 읽는 ConfigMap 들이다. configMap 하나든 projected 로
// 여럿을 합쳤든 같은 모양으로 돌려준다.
func (v volume) sources() []cmSource {
	if v.ConfigMap != nil {
		return []cmSource{*v.ConfigMap}
	}
	if v.Projected == nil {
		return nil
	}
	var out []cmSource
	for _, s := range v.Projected.Sources {
		if s.ConfigMap != nil {
			out = append(out, *s.ConfigMap)
		}
	}
	return out
}

type k8sObject struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name   string            `yaml:"name"`
		Labels map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
	Data map[string]string `yaml:"data"`
	// 파비콘(.ico)은 바이트 그대로여야 해서 base64 로 실린다.
	BinaryData map[string]string `yaml:"binaryData"`
	Spec       struct {
		Template struct {
			Spec struct {
				Containers []struct {
					VolumeMounts []struct {
						Name      string `yaml:"name"`
						MountPath string `yaml:"mountPath"`
						SubPath   string `yaml:"subPath"`
					} `yaml:"volumeMounts"`
				} `yaml:"containers"`
				Volumes []volume `yaml:"volumes"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

func decodeAll(t *testing.T, rendered string) []k8sObject {
	t.Helper()
	dec := yaml.NewDecoder(bytes.NewReader([]byte(rendered)))
	var objs []k8sObject
	for {
		var obj k8sObject
		err := dec.Decode(&obj)
		if err == io.EOF {
			return objs
		}
		if err != nil {
			t.Fatalf("렌더 결과를 읽지 못했다: %v", err)
		}
		objs = append(objs, obj)
	}
}

// themeData 는 테마 ConfigMap 들이 나르는 파일을 한데 모은다.
func themeData(t *testing.T, objs []k8sObject) map[string]string {
	t.Helper()
	data := map[string]string{}
	for _, obj := range objs {
		if obj.Kind != "ConfigMap" || obj.Metadata.Labels["app.kubernetes.io/component"] != "keycloak-theme" {
			continue
		}
		for k, v := range obj.Data {
			data[k] = v
		}
		for k, v := range obj.BinaryData {
			raw, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				t.Errorf("binaryData %q 가 base64 가 아니다: %v", k, err)
				continue
			}
			data[k] = string(raw)
		}
	}
	if len(data) == 0 {
		t.Fatal("테마 ConfigMap 이 렌더되지 않았다")
	}
	return data
}

// containerPath 는 테마 파일이 파드 안에서 놓여야 하는 자리다.
func containerPath(root string, f themeFile) string {
	if f.dir == "." {
		return root + "/" + f.name
	}
	return root + "/" + f.dir + "/" + f.name
}

func TestKeycloakTheme_ConfigMapsCarryEveryThemeFileVerbatim(t *testing.T) {
	// 파일이 실렸는지만이 아니라 그대로 실렸는지 본다. CSS·SVG·메시지 번들은
	// 블록 스칼라로 들어가는데, 들여쓰기가 어긋나면 렌더는 성공하고 내용만
	// 뭉개진다.
	data := themeData(t, decodeAll(t, renderChart(t)))

	for _, f := range themeFiles(t) {
		want, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatalf("%s 를 읽지 못했다: %v", f.path, err)
		}
		got, ok := data[f.name]
		if !ok {
			t.Errorf("테마 파일 %q 가 어느 ConfigMap 에도 없다 — 파드에 안 들어간다", f.name)
			continue
		}
		// 블록 스칼라 "|-" 는 끝의 개행 하나를 떨군다.
		if got != string(want) && got+"\n" != string(want) {
			t.Errorf("테마 파일 %q 의 내용이 원본과 다르다 (원본 %d바이트, 렌더 %d바이트)",
				f.name, len(want), len(got))
		}
	}
}

func TestKeycloakTheme_EveryFileLandsWhereKeycloakLooks(t *testing.T) {
	// 폴더가 하나 늘었는데 마운트를 안 붙이면, Keycloak 은 그 폴더만 빈 채로
	// 화면을 낸다 — 오류 없이 기본 문구로 되돌아가므로 눈으로 봐야 안다.
	//
	// 마운트가 있는지만 봐서는 부족하다. ConfigMap 키에는 "/" 를 못 넣으므로
	// 하위 폴더의 파일은 아무 것도 안 하면 **한 단 위에 평평하게** 떨어진다.
	// 마운트 지점부터 볼륨이 실제로 놓는 경로까지 따라가 본다.
	objs := decodeAll(t, renderChart(t))
	themeRoot := themeRootFor(t)

	// ConfigMap 이름 → 그 안의 키
	keys := map[string][]string{}
	for _, obj := range objs {
		if obj.Kind != "ConfigMap" {
			continue
		}
		for k := range obj.Data {
			keys[obj.Metadata.Name] = append(keys[obj.Metadata.Name], k)
		}
		for k := range obj.BinaryData {
			keys[obj.Metadata.Name] = append(keys[obj.Metadata.Name], k)
		}
	}

	vols := map[string]volume{}
	for _, obj := range objs {
		for _, v := range obj.Spec.Template.Spec.Volumes {
			vols[v.Name] = v
		}
	}

	// 파드 안에 실제로 생길 파일 경로
	placed := map[string]bool{}
	mounted := 0
	for _, obj := range objs {
		for _, ctr := range obj.Spec.Template.Spec.Containers {
			for _, m := range ctr.VolumeMounts {
				if !strings.HasPrefix(m.MountPath, themeRoot) {
					continue
				}
				mounted++
				vol, ok := vols[m.Name]
				if !ok {
					t.Errorf("마운트 %q 가 볼륨 %q 를 쓰는데 그런 볼륨이 없다",
						m.MountPath, m.Name)
					continue
				}
				if m.SubPath != "" {
					// 파일 하나만 골라 붙이는 마운트다.
					placed[m.MountPath] = true
					continue
				}
				for _, src := range vol.sources() {
					if len(src.Items) > 0 {
						for _, it := range src.Items {
							placed[m.MountPath+"/"+it.Path] = true
						}
						continue
					}
					// items 가 없으면 키 이름 그대로, 한 단에 평평하게 놓인다.
					for _, k := range keys[src.Name] {
						placed[m.MountPath+"/"+k] = true
					}
				}
			}
		}
	}
	if mounted == 0 {
		t.Fatal("테마 마운트가 하나도 없다")
	}

	var missing []string
	for _, f := range themeFiles(t) {
		if p := containerPath(themeRoot, f); !placed[p] {
			missing = append(missing, p)
		}
	}
	sort.Strings(missing)
	for _, p := range missing {
		t.Errorf("%q 에 파일이 놓이지 않는다 — 그 파일은 파드에 안 나타난다", p)
	}
}

func TestKeycloakTheme_VolumesReferenceConfigMapsThatExist(t *testing.T) {
	// nullus.fullname 같은 헬퍼를 서브차트 values 에 그대로 쓰면 .Chart 와
	// .Values 가 서브차트 것으로 잡혀 다른 이름이 나온다. 이름이 어긋나면 파드가
	// ConfigMap 을 못 찾아 뜨지 않는다 — 렌더만 봐서는 둘 다 그럴듯해 보인다.
	objs := decodeAll(t, renderChart(t))

	existing := map[string]bool{}
	for _, obj := range objs {
		if obj.Kind == "ConfigMap" {
			existing[obj.Metadata.Name] = true
		}
	}

	referenced := 0
	for _, obj := range objs {
		for _, vol := range obj.Spec.Template.Spec.Volumes {
			if !strings.HasPrefix(vol.Name, "nullus-login-theme") {
				continue
			}
			for _, src := range vol.sources() {
				referenced++
				if !existing[src.Name] {
					t.Errorf("볼륨 %q 가 %q 를 찾는데 그런 ConfigMap 이 없다",
						vol.Name, src.Name)
				}
			}
		}
	}
	if referenced == 0 {
		t.Fatal("테마 볼륨이 하나도 선언되지 않았다")
	}
}

func TestKeycloakTheme_DisabledWhenKeycloakIsExternal(t *testing.T) {
	// 외부 IdP(BYO) 모드에서는 우리가 띄우는 Keycloak 이 없다. 테마 ConfigMap 만
	// 덩그러니 남으면 아무도 안 읽는 리소스가 클러스터에 쌓인다.
	rendered := renderChart(t, "--set", "keycloak.enabled=false")

	if strings.Contains(rendered, themeRootFor(t)) {
		t.Error("keycloak.enabled=false 인데 테마 배선이 남았다")
	}
}

func TestKeycloakTheme_ThemeNameMatchesFolder(t *testing.T) {
	// 차트가 나르는 폴더 이름과 realm 이 가리키는 테마 이름이 갈라지면 Keycloak
	// 은 오류 없이 기본 화면으로 되돌아간다.
	dir := filepath.Join(chartDir(t), "files/keycloak-theme", themeName)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("테마 폴더 이름이 %q 와 다르다: %v", themeName, err)
	}
}

func TestKeycloakTheme_GeneratedArtIsUpToDate(t *testing.T) {
	// 일러스트와 그 좌표는 scripts/emit-keycloak-theme-art.py 가 뽑는다. 손으로
	// 고치거나 고친 뒤 다시 뽑는 걸 잊으면, 정지 그림과 그 위에 얹히는 조각의
	// 좌표가 갈라져 크레인이 허공에 짐을 놓는다 — 렌더도 테스트도 통과하므로
	// 눈으로 보기 전엔 모른다.
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 이 PATH 에 없다 — 생성물 검증을 건너뛴다")
	}

	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("저장소 경로: %v", err)
	}
	script := filepath.Join(repo, "scripts/emit-keycloak-theme-art.py")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("생성기를 찾지 못했다: %v", err)
	}

	out := t.TempDir()
	if b, err := exec.Command(python, script, out).CombinedOutput(); err != nil {
		t.Fatalf("생성기 실행 실패: %v\n%s", err, b)
	}

	committed := filepath.Join(chartDir(t), "files/keycloak-theme", themeName, "login/resources")
	fresh, err := os.ReadDir(out)
	if err != nil {
		t.Fatalf("생성 결과를 읽지 못했다: %v", err)
	}
	for _, e := range fresh {
		want, err := os.ReadFile(filepath.Join(out, e.Name()))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		got, err := os.ReadFile(filepath.Join(committed, e.Name()))
		if err != nil {
			t.Errorf("생성물 %q 가 저장소에 없다 — 생성기를 다시 실행한다", e.Name())
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("생성물 %q 가 생성기 출력과 다르다 — "+
				"python3 scripts/emit-keycloak-theme-art.py 를 다시 실행한다", e.Name())
		}
	}
}
