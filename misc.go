package libgc

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt"
	"github.com/grandcat/zeroconf"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 为Gin router 添加CRUD
func APIBuilder(handlers ...func(*gin.RouterGroup) *gin.RouterGroup) func(gin.IRouter, string) *gin.RouterGroup {
	return func(router gin.IRouter, path string) *gin.RouterGroup {
		group := router.Group(path)
		for _, handler := range handlers {
			group = handler(group)
		}
		return group
	}
}

// 生成token
func GenerateToken(userClaim *UserClaim) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, userClaim)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// 解析token
func ParseToken(token string) (*UserClaim, error) {
	userClaim := new(UserClaim)
	claim, err := jwt.ParseWithClaims(token, userClaim, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if !claim.Valid {
		return nil, errors.New("token validation failed")
	}
	return userClaim, nil
}

// 验证token
func (uc *UserClaim) Valid() error {
	// 过期时间为一周
	if time.Now().Unix() > uc.Expire.Unix() {
		return errors.New("token expired")
	}
	return nil
}

// 验证权限
// 权限位于[permLo, permHi]之间则为合理
func AuthPermission(permLo, permHi int) func(c *gin.Context, token UserClaim) error {
	return func(c *gin.Context, token UserClaim) error {
		if token.Permission < permLo || token.Permission > permHi {
			c.AbortWithStatus(http.StatusForbidden)
			return errors.New("permission denied")
		}
		c.Set("name", token.Name)
		c.Set("permission", token.Permission)
		c.Next()
		return nil
	}
}

// 生成uuid
func GenerateUUID() string {
	uuid, err := uuid.NewV4()
	if err != nil {
		return ""
	}
	return uuid.String()
}

// 生成验证码
// 返回验证码id，验证码
func GenerateCaptcha(length int, charset string) (string, string) {
	if charset == "" {
		charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	}
	return GenerateUUID(), func() string {
		rand.NewSource(time.Now().UnixNano())
		result := make([]byte, length)
		for i := range result {
			result[i] = charset[rand.Intn(len(charset))]
		}
		return string(result)
	}()
}

// 验证验证码
func VerifyCaptcha(id string, captcha string, db *redis.Client) bool {
	ctx := context.Background()
	value := db.Get(ctx, id).String()
	return captcha == value
}

func NewRedis(config *RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
	})
}

/*
	 连接数据库
	 config: 数据库配置
	 migrator: 数据库迁移函数, 为nil则不迁移
	 gormConfig: gorm配置
	 迁移函数示例:

		func Migrate(db *gorm.DB) error {
			return db.AutoMigrate(&User{})
		}
*/
func NewDB(config *DatabaseConfig, migrator func(*gorm.DB) error, gormConfig *gorm.Config) *gorm.DB {
	var db *gorm.DB
	var err error
	if gormConfig == nil {
		gormConfig = &gorm.Config{}
	}
	switch config.Type {
	case "mysql":
		dsn := config.User + ":" + config.Password + "@tcp(" + config.Host + ":" + config.Port + ")/" + config.DB + "?charset=utf8mb4&parseTime=True&loc=Local"
		db, err = gorm.Open(mysql.Open(dsn), gormConfig)
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(config.DB), gormConfig)
	}
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	if config.Migrate {
		if migrator == nil {
			log.Fatalf("Migrator is nil")
		}
		if err = migrator(db); err != nil {
			log.Fatalf("Failed to migrate tables: %v", err)
		}
	}
	return db
}

func HashedPassword(password string) string {
	res, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("Failed to hash password: ", err)
	}
	return string(res)

}
func CheckPasswordHash(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// LoadCSV loads data from a CSV file and returns a slice of maps representing the rows.
func LoadCSV(filePath string) (map[string]map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	headers := rows[0]
	data := make(map[string]map[string]string, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		id := row[0]
		values := make(map[string]string)
		for j := 1; j < len(row); j++ {
			values[headers[j]] = row[j]
		}
		data[id] = values
	}
	return data, nil
}

func GenerateShortLink(url string) string {
	h := sha256.New()
	h.Write([]byte(url))
	hash := h.Sum(nil)
	shortLink := hex.EncodeToString(hash)[:24] // 取前8个字符作为短链接
	return shortLink
}

// new logger, default to append mode
func NewLoger(logFile string) *log.Logger {
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	return log.New(file, "", log.LstdFlags)
}

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Println("Failed to get local IP address: ", err)
		return ""
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return ""
}

func RandPort() int {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	return rand.Intn(65535-1024) + 1024
}

// mDNS广播
func RunmDnsBroadcast(serviceName, serviceDomain, instanceName string, text []string, servicePort int) {
	server, err := zeroconf.Register(instanceName, serviceName, serviceDomain, servicePort, text, nil)
	if err != nil {
		log.Fatalf("Failed to register mDNS service: %v", err)
	}
	defer server.Shutdown()
	log.Printf("mDNS service %s.%s:%d published", instanceName, serviceName, servicePort)
	select {}
}
