package system

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

var (
	cpuDirRegExp = regexp.MustCompile(`/cpu(\d+)`)
	nodeDir      = "/sys/devices/system/node"
	cpusPath     = "/sys/devices/system/cpu"
)

const (
	sysFsCPUTopology = "/topology"
	cpuDirPattern    = "cpu*[0-9]"
	nodeDirPattern   = "node*[0-9]"

	coreIDFilePath    = sysFsCPUTopology + "/core_id"
	packageIDFilePath = sysFsCPUTopology + "/physical_package_id"
)

type core struct {
	Id       int   `json:"core_id"`
	Threads  []int `json:"thread_ids"`
	SocketID int   `json:"socket_id"`
}

func init() {
	//cannot copy file to /sys for doc build, use this as a hack for testing
	cpuDirEnv := os.Getenv("CPU_DIR")
	if cpuDirEnv != "" {
		cpusPath = cpuDirEnv
	}
	nodeDirEnv := os.Getenv("NODE_DIR")
	if nodeDirEnv != "" {
		nodeDir = nodeDirEnv
	}
}

// GetMilliNumCores this is a striped version of GetNodesInfo that only get information for NumCores
// https://github.com/google/cadvisor/blob/master/utils/sysinfo/sysinfo.go#L203
func GetMilliNumCores() (int, error) {
	allLogicalCoresCount := 0

	nodesDirs, err := getNodesPaths()
	if err != nil {
		return 0, err
	}
	if len(nodesDirs) == 0 {
		zap.L().Error("Nodes topology is not available, providing CPU topology")
		cpuCount, err := getCPUCount()
		if err != nil {
			return 0, err
		}
		return cpuCount * 1000, nil
	}

	for _, dir := range nodesDirs {
		cpuDirs, err := getCPUsPaths(dir)
		if len(cpuDirs) == 0 {
			zap.L().Error("Found node without any CPU", zap.String("dir", dir), zap.Error(err))
		} else {
			cores, err := getCoresInfo(cpuDirs)
			if err != nil {
				return 0, err
			}
			for _, core := range cores {
				allLogicalCoresCount += len(core.Threads)
			}
		}

	}
	return allLogicalCoresCount * 1000, err

}

func getCPUCount() (int, error) {
	cpusPaths, err := getCPUsPaths(cpusPath)
	if err != nil {
		return 0, err
	}
	cpusCount := len(cpusPaths)

	if cpusCount == 0 {
		return 0, fmt.Errorf("Any CPU is not available, cpusPath: %s", cpusPath)
	}
	return cpusCount, nil
}

func getNodesPaths() ([]string, error) {
	pathPattern := fmt.Sprintf("%s/%s", nodeDir, nodeDirPattern)
	return filepath.Glob(pathPattern)
}

func getCPUsPaths(cpusPath string) ([]string, error) {
	pathPattern := fmt.Sprintf("%s/%s", cpusPath, cpuDirPattern)
	return filepath.Glob(pathPattern)
}

func getCPUPhysicalPackageID(cpuPath string) (string, error) {
	packageIDFilePath := fmt.Sprintf("%s%s", cpuPath, packageIDFilePath)
	// #nosec G304 // This cpuPath essentially come from getCPUsPaths and it should be a system path
	packageID, err := os.ReadFile(packageIDFilePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(packageID)), err
}

func getCoresInfo(cpuDirs []string) ([]core, error) {
	cores := make([]core, 0, len(cpuDirs))
	for _, cpuDir := range cpuDirs {
		cpuID, err := getCPUID(cpuDir)
		if err != nil {
			return nil, fmt.Errorf("unexpected format of CPU directory, cpuDirRegExp %s, cpuDir: %s", cpuDirRegExp, cpuDir)
		}
		if !IsCPUOnline(cpuID) {
			continue
		}

		rawPhysicalID, err := getCoreID(cpuDir)
		if os.IsNotExist(err) {
			zap.L().Warn("Cannot read core id for input cpuDir, core_id file does not exist",
				zap.String("cpuDir", cpuDir), zap.Error(err))
			continue
		} else if err != nil {
			return nil, err
		}
		physicalID, err := strconv.Atoi(rawPhysicalID)
		if err != nil {
			return nil, err
		}

		rawPhysicalPackageID, err := getCPUPhysicalPackageID(cpuDir)
		if os.IsNotExist(err) {
			zap.L().Warn("Cannot read physical package id for input cpuDir, physical_package_id file does not exist",
				zap.String("cpuDir", cpuDir), zap.Error(err))
			continue
		} else if err != nil {
			return nil, err
		}

		physicalPackageID, err := strconv.Atoi(rawPhysicalPackageID)
		if err != nil {
			return nil, err
		}

		coreIDx := -1
		for id, core := range cores {
			if core.Id == physicalID && core.SocketID == physicalPackageID {
				coreIDx = id
			}
		}
		if coreIDx == -1 {
			cores = append(cores, core{})
			coreIDx = len(cores) - 1
		}
		desiredCore := &cores[coreIDx]

		desiredCore.Id = physicalID
		desiredCore.SocketID = physicalPackageID

		if len(desiredCore.Threads) == 0 {
			desiredCore.Threads = []int{cpuID}
		} else {
			desiredCore.Threads = append(desiredCore.Threads, cpuID)
		}

	}
	return cores, nil
}

func getCPUID(str string) (int, error) {
	matches := cpuDirRegExp.FindStringSubmatch(str)
	if len(matches) != 2 {
		return 0, fmt.Errorf("failed to match regexp, str: %s", str)
	}
	valInt, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}
	return valInt, nil
}

func getCoreID(cpuPath string) (string, error) {
	coreIDFilePath := fmt.Sprintf("%s%s", cpuPath, coreIDFilePath)
	// #nosec G304 // This cpuPath essentially come from getCPUsPaths and it should be a system path
	coreID, err := os.ReadFile(coreIDFilePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(coreID)), err
}

func IsCPUOnline(cpuID int) bool {
	cpuOnlinePath, err := filepath.Abs(cpusPath + "/online")
	if err != nil {
		zap.L().Info("Unable to get absolute path", zap.String("absolutPath", cpusPath+"/online"))
		return false
	}

	// Quick check to determine if file exists: if it does not then kernel CPU hotplug is disabled and all CPUs are online.
	_, err = os.Stat(cpuOnlinePath)
	if err != nil && os.IsNotExist(err) {
		return true
	}
	if err != nil {
		zap.L().Warn("Unable to stat cpuOnlinePath",
			zap.String("cpuOnlinePath", cpuOnlinePath),
			zap.Error(err))
	}

	isOnline, err := isCpuOnline(cpuOnlinePath, uint16(cpuID))
	if err != nil {
		zap.L().Error("Unable to get online CPUs list", zap.Error(err))
		return false
	}
	return isOnline
}

func isCpuOnline(path string, cpuID uint16) (bool, error) {
	// #nosec G304 // This path is cpuOnlinePath from isCPUOnline
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if len(fileContent) == 0 {
		return false, fmt.Errorf("%s found to be empty", path)
	}

	cpuList := strings.TrimSpace(string(fileContent))
	for _, s := range strings.Split(cpuList, ",") {
		splitted := strings.SplitN(s, "-", 3)
		switch len(splitted) {
		case 3:
			return false, fmt.Errorf("invalid values in %s", path)
		case 2:
			min, err := strconv.ParseUint(splitted[0], 10, 16)
			if err != nil {
				return false, err
			}
			max, err := strconv.ParseUint(splitted[1], 10, 16)
			if err != nil {
				return false, err
			}
			if min > max {
				return false, fmt.Errorf("invalid values in %s", path)
			}
			// Return true, if the CPU under consideration is in the range of online CPUs.
			if cpuID >= uint16(min) && cpuID <= uint16(max) {
				return true, nil
			}
		case 1:
			value, err := strconv.ParseUint(s, 10, 16)
			if err != nil {
				return false, err
			}
			if uint16(value) == cpuID {
				return true, nil
			}
		}
	}

	return false, nil
}
