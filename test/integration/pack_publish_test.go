package integration

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gpm.sh/gpm/gpm-cli/internal/config"
)

type PackPublishSuite struct {
	suite.Suite
	tmpDir     string
	gpmBinary  string
	testServer string
}

func (s *PackPublishSuite) SetupSuite() {
	tmpDir := s.T().TempDir()
	s.tmpDir = tmpDir

	// Add .exe extension on Windows
	binaryName := "gpm"
	if filepath.Ext(os.Args[0]) == ".exe" {
		binaryName = "gpm.exe"
	}
	s.gpmBinary = filepath.Join(tmpDir, binaryName)

	workingDir, err := os.Getwd()
	require.NoError(s.T(), err)

	mainGoPath := filepath.Join(workingDir, "..", "..", "main.go")

	buildCmd := exec.Command("go", "build", "-o", s.gpmBinary, mainGoPath)
	buildCmd.Dir = filepath.Join(workingDir, "..", "..")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		s.T().Skipf("Failed to build binary for integration tests: %v\nOutput: %s", err, output)
	}

	config.InitConfig()
	cfg := config.GetConfig()
	s.testServer = cfg.Registry
}

func (s *PackPublishSuite) TearDownSuite() {
	os.RemoveAll(s.tmpDir)
}

func (s *PackPublishSuite) SetupTest() {
	testDir := filepath.Join(s.tmpDir, "test-package")
	require.NoError(s.T(), os.MkdirAll(testDir, 0755))
	require.NoError(s.T(), os.Chdir(testDir))
}

func (s *PackPublishSuite) TestPackCommand() {
	packageJSON := `{
		"name": "com.integration.test-package",
		"version": "1.0.0",
		"displayName": "Integration Test Package",
		"description": "A test package for integration testing",
		"unity": "2022.3",
		"license": "MIT"
	}`
	require.NoError(s.T(), os.WriteFile("package.json", []byte(packageJSON), 0644))

	require.NoError(s.T(), os.MkdirAll("Runtime/Scripts", 0755))
	script := `using UnityEngine;

namespace IntegrationTest
{
    public class TestScript : MonoBehaviour
    {
        void Start()
        {
            Debug.Log("Integration test package loaded");
        }
    }
}`
	require.NoError(s.T(), os.WriteFile("Runtime/Scripts/TestScript.cs", []byte(script), 0644))

	cmd := exec.Command(s.gpmBinary, "pack")
	output, err := cmd.CombinedOutput()
	require.NoError(s.T(), err, "Pack command failed: %s", string(output))

	tarball := "com.integration.test-package-1.0.0.tgz"
	assert.FileExists(s.T(), tarball)

	outputStr := string(output)
	assert.Contains(s.T(), outputStr, "GPM Package Created Successfully")
	assert.Contains(s.T(), outputStr, "com.integration.test-package")
	assert.Contains(s.T(), outputStr, "1.0.0")
	assert.Contains(s.T(), outputStr, tarball)
}

func (s *PackPublishSuite) TestPackPublishWorkflow() {
	if !s.isServerAvailable() {
		s.T().Skip("Test server not available")
	}

	s.createTestPackage()

	cmd := exec.Command(s.gpmBinary, "pack")
	_, err := cmd.CombinedOutput()
	require.NoError(s.T(), err)

	cmd = exec.Command(s.gpmBinary, "config", "set", "registry", s.testServer)
	_, err = cmd.CombinedOutput()
	require.NoError(s.T(), err)

	tarball := "com.integration.test-package-1.0.0.tgz"
	cmd = exec.Command(s.gpmBinary, "publish", tarball)
	output, _ := cmd.CombinedOutput()

	outputStr := string(output)
	assert.Contains(s.T(), outputStr, "Publishing Package")
}

func (s *PackPublishSuite) createTestPackage() {
	packageJSON := `{
		"name": "com.integration.test-package",
		"version": "1.0.0",
		"displayName": "Integration Test Package",
		"description": "A test package for integration testing",
		"unity": "2022.3",
		"license": "MIT"
	}`
	require.NoError(s.T(), os.WriteFile("package.json", []byte(packageJSON), 0644))

	require.NoError(s.T(), os.MkdirAll("Runtime/Scripts", 0755))
	script := `using UnityEngine;
public class TestScript : MonoBehaviour { }`
	require.NoError(s.T(), os.WriteFile("Runtime/Scripts/TestScript.cs", []byte(script), 0644))
}

func (s *PackPublishSuite) isServerAvailable() bool {
	resp, err := http.Get(s.testServer + "/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func TestPackPublishSuite(t *testing.T) {
	suite.Run(t, new(PackPublishSuite))
}
