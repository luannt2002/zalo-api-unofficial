package testenv

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Identities chứa các định danh dùng trong test được nạp từ biến môi trường (hoặc .env.test).
type Identities struct {
	Send2meA     string
	Send2meB     string
	Send2meC     string
	UIDA         string
	UIDEntity    string
	UID9         string
	GIDA         string
	ExpectedMask uint64
	NoiseA       int64
}

var (
	loadOnce sync.Once
	cachedID *Identities
	loadErr  error
)

// Load nạp và xác thực các ID dùng cho test suite từ environment hoặc file .env.test.
// Nếu t != nil, tự động fail test với thông báo lỗi rõ ràng nếu cấu hình không hợp lệ.
func Load(t *testing.T) *Identities {
	loadOnce.Do(func() {
		cachedID, loadErr = loadAndVerify()
	})

	if loadErr != nil {
		if t != nil {
			t.Fatalf("testenv: failed to load test environment: %v", loadErr)
		}
		panic(loadErr)
	}

	return cachedID
}

// loadAndVerify tìm và nạp biến môi trường, sau đó kiểm tra tính hợp lệ của toán học và kiểu dữ liệu.
func loadAndVerify() (*Identities, error) {
	// Nếu chưa có biến môi trường thì tự động tìm nạp từ .env.test
	if os.Getenv("TEST_ZALO_SEND2ME_A") == "" {
		if err := loadDotEnv(); err != nil {
			return nil, fmt.Errorf("load .env.test: %w", err)
		}
	}

	id := &Identities{
		Send2meA:  strings.TrimSpace(os.Getenv("TEST_ZALO_SEND2ME_A")),
		Send2meB:  strings.TrimSpace(os.Getenv("TEST_ZALO_SEND2ME_B")),
		Send2meC:  strings.TrimSpace(os.Getenv("TEST_ZALO_SEND2ME_C")),
		UIDA:      strings.TrimSpace(os.Getenv("TEST_ZALO_UID_A")),
		UIDEntity: strings.TrimSpace(os.Getenv("TEST_ZALO_UID_ENTITY")),
		UID9:      strings.TrimSpace(os.Getenv("TEST_ZALO_UID9")),
		GIDA:      strings.TrimSpace(os.Getenv("TEST_ZALO_GIDA")),
	}

	rawMask := strings.TrimSpace(os.Getenv("TEST_ZALO_EXPECTED_MASK"))
	if rawMask != "" {
		mask, err := parseUint64Flexible(rawMask)
		if err != nil {
			return nil, fmt.Errorf("invalid TEST_ZALO_EXPECTED_MASK %q: %w", rawMask, err)
		}
		id.ExpectedMask = mask
	}

	rawNoise := strings.TrimSpace(os.Getenv("TEST_ZALO_NOISE_A"))
	if rawNoise != "" {
		noise, err := strconv.ParseInt(rawNoise, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid TEST_ZALO_NOISE_A %q: %w", rawNoise, err)
		}
		id.NoiseA = noise
	}

	if err := id.Verify(); err != nil {
		return nil, fmt.Errorf("verify test identities: %w", err)
	}

	return id, nil
}

// Verify kiểm tra tính toàn vẹn của dữ liệu test:
// 1. Các ID không được rỗng và phải là chuỗi số nguyên dương 64-bit hợp lệ
// 2. Mối quan hệ toán học XOR Mask giữa Send2meA và Send2meB phải khớp với ExpectedMask
func (id *Identities) Verify() error {
	if id.Send2meA == "" {
		return fmt.Errorf("missing TEST_ZALO_SEND2ME_A")
	}
	if id.Send2meB == "" {
		return fmt.Errorf("missing TEST_ZALO_SEND2ME_B")
	}
	if id.UIDA == "" {
		return fmt.Errorf("missing TEST_ZALO_UID_A")
	}

	valA, err := strconv.ParseUint(id.Send2meA, 10, 64)
	if err != nil {
		return fmt.Errorf("parse TEST_ZALO_SEND2ME_A %q: %w", id.Send2meA, err)
	}

	valB, err := strconv.ParseUint(id.Send2meB, 10, 64)
	if err != nil {
		return fmt.Errorf("parse TEST_ZALO_SEND2ME_B %q: %w", id.Send2meB, err)
	}

	if id.Send2meC != "" {
		if _, err := strconv.ParseUint(id.Send2meC, 10, 64); err != nil {
			return fmt.Errorf("parse TEST_ZALO_SEND2ME_C %q: %w", id.Send2meC, err)
		}
	}

	if _, err := strconv.ParseUint(id.UIDA, 10, 64); err != nil {
		return fmt.Errorf("parse TEST_ZALO_UID_A %q: %w", id.UIDA, err)
	}

	if id.UIDEntity != "" {
		if _, err := strconv.ParseUint(id.UIDEntity, 10, 64); err != nil {
			return fmt.Errorf("parse TEST_ZALO_UID_ENTITY %q: %w", id.UIDEntity, err)
		}
	}

	if id.UID9 != "" {
		if _, err := strconv.ParseUint(id.UID9, 10, 64); err != nil {
			return fmt.Errorf("parse TEST_ZALO_UID9 %q: %w", id.UID9, err)
		}
	}

	if id.GIDA != "" {
		if len(id.GIDA) != 32 {
			return fmt.Errorf("invalid TEST_ZALO_GIDA length %d (expected 32)", len(id.GIDA))
		}
	}

	if id.NoiseA != 0 && id.NoiseA >= 0 {
		return fmt.Errorf("invalid TEST_ZALO_NOISE_A %d (expected negative int64 with bit 63 = 1)", id.NoiseA)
	}

	computedMask := valA ^ valB
	if id.ExpectedMask != 0 && computedMask != id.ExpectedMask {
		return fmt.Errorf("mask mismatch: Send2meA (%d) ^ Send2meB (%d) = 0x%x, but ExpectedMask is 0x%x",
			valA, valB, computedMask, id.ExpectedMask)
	}

	id.ExpectedMask = computedMask
	return nil
}

// loadDotEnv tìm file .env.test (hoặc .env.test.example) bằng cách đi ngược lên các thư mục cha
func loadDotEnv() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	for {
		target := filepath.Join(dir, ".env.test")
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			return parseEnvFile(target)
		}

		exampleTarget := filepath.Join(dir, ".env.test.example")
		if info, err := os.Stat(exampleTarget); err == nil && !info.IsDir() {
			return parseEnvFile(exampleTarget)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return nil
}

// parseEnvFile đọc file env và gán vào môi trường nếu chưa có
func parseEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)

		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}

	return scanner.Err()
}

func parseUint64Flexible(s string) (uint64, error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}

