package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Keycloak   KeycloakConfig   `mapstructure:"keycloak"`
	Helm       HelmConfig       `mapstructure:"helm"`
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
	Log        LogConfig        `mapstructure:"log"`
	Platform   PlatformConfig   `mapstructure:"platform"`
	Backup     BackupConfig     `mapstructure:"backup"`
}

// BackupConfig 는 백업/복구 설정이다.
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md (nullus-plan#75)
//
// 목적지는 **대상 클러스터 밖**의 S3 호환 오브젝트 스토리지다. 클러스터가
// 통째로 사라져도 백업본은 남아야 하기 때문이다(§4.2).
//
// 자격증명이 스택 OpenBao 가 아니라 여기(=컨트롤 플레인 설정)에서 오는 것이
// 핵심이다. 금고에서 조달하면 스택이 죽을 때 백업본이 멀쩡해도 가져올 수
// 없는 순환이 생긴다(§4.2.1).
type BackupConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// SealKey 는 산출물 암호화 키다. 정확히 32바이트여야 한다.
	// ENCRYPTION_KEY 와 **다른 값**이어야 한다 — 같으면 키 하나를 잃고
	// 둘 다 잃는다(§5.2).
	SealKey   string `mapstructure:"seal_key"`
	SealKeyID string `mapstructure:"seal_key_id"`

	Destination BackupDestinationConfig `mapstructure:"destination"`
	Schedule    BackupScheduleConfig    `mapstructure:"schedule"`
	Retention   BackupRetentionConfig   `mapstructure:"retention"`

	// KeycloakDatabase 는 Keycloak 전용 DB 다. 배포 경로에 따라 위치가
	// 다르므로(차트 서브차트 / 에어갭 독립 릴리스) 설정으로 받는다(§1.2).
	KeycloakDatabase BackupDatabaseConfig `mapstructure:"keycloak_database"`
}

type BackupDestinationConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	Bucket    string `mapstructure:"bucket"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Region    string `mapstructure:"region"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	Prefix    string `mapstructure:"prefix"`
}

type BackupDatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

type BackupScheduleConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// Interval 기본값은 24시간이다 (RPO 24시간, §2).
	Interval  time.Duration `mapstructure:"interval"`
	OrgID     string        `mapstructure:"org_id"`
	StackID   string        `mapstructure:"stack_id"`
	Namespace string        `mapstructure:"namespace"`
	Mode      string        `mapstructure:"mode"`
}

type BackupRetentionConfig struct {
	Daily         int   `mapstructure:"daily"`
	Weekly        int   `mapstructure:"weekly"`
	Monthly       int   `mapstructure:"monthly"`
	MaxTotalBytes int64 `mapstructure:"max_total_bytes"`
}

// PlatformConfig 는 플랫폼 자신이 어디에 떠 있는지를 담는다.
//
// 스택을 플랫폼과 같은 네임스페이스에 설치하면 Helm 소유권이 충돌하고, 스택을
// 지울 때 플랫폼 리소스까지 지워진다 — 2026-08-20 에 실제로 그렇게 nullus.io 가
// 통째로 내려갔다. 자기 자리를 알아야 그 자리를 지킬 수 있다.
//
// 차트가 Downward API 로 NULLUS_PLATFORM_NAMESPACE 를 넣어 준다. 클러스터 밖에서
// 도는 개발 환경에서는 비어 있고, 그때는 이 검사를 하지 않는다.
type PlatformConfig struct {
	Namespace string `mapstructure:"namespace"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Name            string        `mapstructure:"name"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Mode    string        `mapstructure:"mode"`
	Session SessionConfig `mapstructure:"session"`
	OIDC    OIDCConfig    `mapstructure:"oidc"`
}

// SessionConfig holds session-based auth configuration.
type SessionConfig struct {
	Secret string `mapstructure:"secret"`
	MaxAge int    `mapstructure:"max_age"`
}

type OIDCConfig struct {
	Provider  string `mapstructure:"provider"`
	IssuerURL string `mapstructure:"issuer_url"`
	Audience  string `mapstructure:"audience"`
}

type KeycloakConfig struct {
	AdminURL string `mapstructure:"admin_url"`
	// PublicURL 은 브라우저가 접근하는 Keycloak 주소다. AdminURL 이 클러스터 내부
	// 주소일 수 있어 따로 둔다. 비우면 auth.oidc.issuer_url 을 물려받는다.
	PublicURL     string `mapstructure:"public_url"`
	Realm         string `mapstructure:"realm"`
	AdminUser     string `mapstructure:"admin_user"`
	AdminPassword string `mapstructure:"admin_password"`
}

type HelmConfig struct {
	Timeout         string `mapstructure:"timeout"`
	NamespacePrefix string `mapstructure:"namespace_prefix"`
}

type PrometheusConfig struct {
	URL string `mapstructure:"url"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// LoadConfig reads configuration from the given file path and environment variables.
func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("NULLUS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindKeycloakAdminEnv(v)
	// 설정 파일에 없는 키는 AutomaticEnv 만으로 잡히지 않는다. 이 값은 차트가
	// Downward API 로만 넣어 주므로 명시적으로 묶는다.
	_ = v.BindEnv("platform.namespace", "NULLUS_PLATFORM_NAMESPACE")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// bindKeycloakAdminEnv 는 SSO 프로비저닝용 Keycloak 자격을 환경변수에 묶는다.
//
// 접두사 없는 KEYCLOAK_* 는 옛 이름이다. main.go 가 os.Getenv("KEYCLOAK_URL") 로
// 직접 읽던 시절의 것이고, cmd/nullus-bootstrap 과 에어갭 스크립트가 아직 같은
// 이름을 쓴다. AutomaticEnv 는 접두사 있는 이름만 보므로 여기서 함께 묶어 둔다 —
// 앞에 적은 이름이 이긴다.
func bindKeycloakAdminEnv(v *viper.Viper) {
	for key, names := range map[string][]string{
		"keycloak.admin_url":      {"NULLUS_KEYCLOAK_ADMIN_URL", "KEYCLOAK_URL"},
		"keycloak.public_url":     {"NULLUS_KEYCLOAK_PUBLIC_URL", "KEYCLOAK_PUBLIC_URL"},
		"keycloak.realm":          {"NULLUS_KEYCLOAK_REALM", "KEYCLOAK_REALM"},
		"keycloak.admin_user":     {"NULLUS_KEYCLOAK_ADMIN_USER", "KEYCLOAK_ADMIN_USER"},
		"keycloak.admin_password": {"NULLUS_KEYCLOAK_ADMIN_PASSWORD", "KEYCLOAK_ADMIN_PASSWORD"},
	} {
		// BindEnv 는 (key, envNames...) 형태라 슬라이스를 펼쳐 넘긴다.
		_ = v.BindEnv(append([]string{key}, names...)...)
	}
}
