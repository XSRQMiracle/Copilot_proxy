package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// deriveMachineKey 从机器指纹派生 AES-256 密钥。
// 使用 SHA-256 确保输出总是 32 字节。
func deriveMachineKey() []byte {
	var material string

	switch runtime.GOOS {
	case "windows":
		material = readWindowsMachineGUID()
	case "linux":
		material = readLinuxMachineID()
	case "darwin":
		material = readMacIOPlatformUUID()
	default:
		// 兜底：使用 hostname（不跨重启持久，但至少每次运行一致）
		host, _ := os.Hostname()
		material = host
	}

	if os.Getenv("COPILOT_PROXY_DEBUG") != "" {
		log.Printf("[DEBUG] deriveMachineKey: platform=%s, material=%q (len=%d)", runtime.GOOS, material, len(material))
	}
	hash := sha256.Sum256([]byte(material))
	return hash[:]
}

func readWindowsMachineGUID() string {
	// reg query "HKLM\SOFTWARE\Microsoft\Cryptography" /v MachineGuid
	out, err := exec.Command("reg", "query", `HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid").Output()
	if err != nil {
		host, _ := os.Hostname()
		log.Printf("[DEBUG] readWindowsMachineGUID: reg query failed (%v), fallback to hostname=%q", err, host)
		return host
	}
	// 输出示例：
	// HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Cryptography
	//     MachineGuid    REG_SZ    xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "REG_SZ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				guid := parts[len(parts)-1]
				log.Printf("[DEBUG] readWindowsMachineGUID: extracted MachineGuid=%q", guid)
				return guid
			}
		}
	}
	host, _ := os.Hostname()
	log.Printf("[DEBUG] readWindowsMachineGUID: REG_SZ line not found in reg output (lines=%d), fallback to hostname=%q", len(lines), host)
	return host
}

func readLinuxMachineID() string {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		data, err = os.ReadFile("/var/lib/dbus/machine-id")
	}
	if err != nil {
		host, _ := os.Hostname()
		return host
	}
	return strings.TrimSpace(string(data))
}

func readMacIOPlatformUUID() string {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		host, _ := os.Hostname()
		return host
	}
	// 搜索 "IOPlatformUUID" = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "IOPlatformUUID") {
			// 提取引号中的值
			start := strings.Index(line, `"`)
			if start < 0 {
				continue
			}
			rest := line[start+1:]
			end := strings.Index(rest, `"`)
			if end < 0 {
				continue
			}
			val := rest[:end]
			// 跳过键名本身
			if strings.EqualFold(val, "IOPlatformUUID") {
				rest2 := rest[end+1:]
				start2 := strings.Index(rest2, `"`)
				if start2 >= 0 {
					rest3 := rest2[start2+1:]
					end2 := strings.Index(rest3, `"`)
					if end2 >= 0 {
						return rest3[:end2]
					}
				}
			}
		}
	}
	host, _ := os.Hostname()
	return host
}

const encryptedPrefix = "enc:v1:"

// EncryptToken 用 AES-GCM 加密明文，返回带 enc:v1: 前缀的 base64 编码密文。
func EncryptToken(plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("cannot encrypt empty token")
	}
	key := deriveMachineKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("rand nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return encryptedPrefix + encoded, nil
}

// DecryptToken 解密 enc:v1: 前缀或传统 base64 编码的 AES-GCM 密文。
// 如果密钥不匹配（机器指纹变化），返回明确的错误。
func DecryptToken(encoded string) (string, error) {
	if encoded == "" {
		return "", errors.New("cannot decrypt empty token")
	}
	// 剥离 enc:v1: 前缀（新格式）；传统格式无前缀直接 base64 解码
	payload := encoded
	if strings.HasPrefix(encoded, encryptedPrefix) {
		payload = strings.TrimPrefix(encoded, encryptedPrefix)
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	key := deriveMachineKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt (machine fingerprint may have changed): %w", err)
	}
	return string(plaintext), nil
}

// IsProbablyEncrypted 判断字符串是否已加密。
// 新版加密使用显式 enc:v1: 前缀；老版加密使用无前缀 base64（通过解码+最小长度校验避免误判）。 
func IsProbablyEncrypted(s string) bool {
	// 显式前缀：新格式
	if strings.HasPrefix(s, encryptedPrefix) {
		return true
	}
	// 长明文 PAT（无前缀 base64）也可能触发启发式；通过 base64 解码验证排除误判
	if len(s) > 40 && strings.TrimSpace(s) == s && !strings.ContainsAny(s, " \t\n") {
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return false
		}
		// AES-GCM 至少 12 字节 nonce + 1 字节密文
		return len(decoded) >= 13
	}
	return false
}
