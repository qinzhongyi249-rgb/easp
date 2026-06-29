package embed

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

// VerifyEmbedSignature 验证嵌入式API签名
// 签名算法: HMAC-SHA256(appSecretHash, sortedKeyValues)
func VerifyEmbedSignature(appSecretHash []byte, appID, timestamp, nonce string, bodyMap map[string]string, signature string) bool {
	// 排序参数
	params := make([]string, 0, 4+len(bodyMap))
	params = append(params, "app_id="+appID)
	params = append(params, "timestamp="+timestamp)
	params = append(params, "nonce="+nonce)
	// body 参数排序
	keys := make([]string, 0, len(bodyMap))
	for k := range bodyMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		params = append(params, k+"="+bodyMap[k])
	}
	// 拼接
	data := strings.Join(params, "\n")
	// HMAC
	mac := hmac.New(sha256.New, appSecretHash)
	mac.Write([]byte(data))
	expectedMAC := mac.Sum(nil)
	expectedHex := hex.EncodeToString(expectedMAC)
	return hmac.Equal([]byte(expectedHex), []byte(signature))
}

// IsOriginAllowed 检查Origin是否在允许列表中
func IsOriginAllowed(origin string, allowedOrigins string) bool {
	if allowedOrigins == "" || allowedOrigins == "*" {
		return true
	}
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return true
	}
	origins := strings.Split(allowedOrigins, ",")
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == origin {
			return true
		}
		// 支持通配符 *.example.com
		if strings.HasPrefix(o, "*.") && strings.HasSuffix(origin, o[1:]) {
			return true
		}
	}
	return false
}

// sha256Hex 返回字符串的sha256十六进制
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// strconvParseInt 封装strconv.Atoi
func strconvParseInt(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
