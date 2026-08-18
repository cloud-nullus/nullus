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
	AdminURL      string `mapstructure:"admin_url"`
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
		"keycloak.realm":          {"NULLUS_KEYCLOAK_REALM", "KEYCLOAK_REALM"},
		"keycloak.admin_user":     {"NULLUS_KEYCLOAK_ADMIN_USER", "KEYCLOAK_ADMIN_USER"},
		"keycloak.admin_password": {"NULLUS_KEYCLOAK_ADMIN_PASSWORD", "KEYCLOAK_ADMIN_PASSWORD"},
	} {
		// BindEnv 는 (key, envNames...) 형태라 슬라이스를 펼쳐 넘긴다.
		_ = v.BindEnv(append([]string{key}, names...)...)
	}
}
