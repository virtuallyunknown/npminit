package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type EmbeddedFile []byte

var (
	//go:embed static/tailwind.config.js
	TailwindConfigJs EmbeddedFile
	//go:embed static/eslintrc.cjs
	EslintConfigJs EmbeddedFile
	//go:embed static/esbuild.js
	EsbuildJs EmbeddedFile
	//go:embed static/index.ts
	IndexTs EmbeddedFile
)

type packageJSON struct {
	Name        string `json:"name"`
	Author      string `json:"author"`
	Description string `json:"description"`
	License     string `json:"license"`
	Version     string `json:"version"`
	Type        string `json:"type"`
}

type tsconfigJSON struct {
	CompilerOptions struct {
		Module                           string `json:"module,omitempty"`
		ModuleResolution                 string `json:"moduleResolution,omitempty"`
		Target                           string `json:"target,omitempty"`
		ForceConsistentCasingInFileNames bool   `json:"forceConsistentCasingInFileNames,omitempty"`
		AllowUnreachableCode             bool   `json:"allowUnreachableCode,omitempty"`
		NoErrorTruncation                bool   `json:"noErrorTruncation,omitempty"`
		EsModuleInterop                  bool   `json:"esModuleInterop,omitempty"`
		IsolatedModules                  bool   `json:"isolatedModules,omitempty"`
		ResolveJSONModule                bool   `json:"resolveJsonModule,omitempty"`
		SkipLibCheck                     bool   `json:"skipLibCheck,omitempty"`
		Jsx                              string `json:"jsx,omitempty"`
		Strict                           bool   `json:"strict,omitempty"`
		NoEmit                           bool   `json:"noEmit,omitempty"`
		RootDir                          string `json:"rootDir,omitempty"`
		OutDir                           string `json:"outDir,omitempty"`
	} `json:"compilerOptions"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

type npmAuditJSON struct {
	Metadata struct {
		Vulnerabilities struct {
			Info     int `json:"info"`
			Low      int `json:"low"`
			Moderate int `json:"moderate"`
			High     int `json:"high"`
			Critical int `json:"critical"`
			Total    int `json:"total"`
		} `json:"vulnerabilities"`
		Dependencies struct {
			Prod         int `json:"prod"`
			Dev          int `json:"dev"`
			Optional     int `json:"optional"`
			Peer         int `json:"peer"`
			PeerOptional int `json:"peerOptional"`
			Total        int `json:"total"`
		} `json:"dependencies"`
	} `json:"metadata"`
}

func generateTsconfigJSON() tsconfigJSON {
	return tsconfigJSON{
		CompilerOptions: struct {
			Module                           string `json:"module,omitempty"`
			ModuleResolution                 string `json:"moduleResolution,omitempty"`
			Target                           string `json:"target,omitempty"`
			ForceConsistentCasingInFileNames bool   `json:"forceConsistentCasingInFileNames,omitempty"`
			AllowUnreachableCode             bool   `json:"allowUnreachableCode,omitempty"`
			NoErrorTruncation                bool   `json:"noErrorTruncation,omitempty"`
			EsModuleInterop                  bool   `json:"esModuleInterop,omitempty"`
			IsolatedModules                  bool   `json:"isolatedModules,omitempty"`
			ResolveJSONModule                bool   `json:"resolveJsonModule,omitempty"`
			SkipLibCheck                     bool   `json:"skipLibCheck,omitempty"`
			Jsx                              string `json:"jsx,omitempty"`
			Strict                           bool   `json:"strict,omitempty"`
			NoEmit                           bool   `json:"noEmit,omitempty"`
			RootDir                          string `json:"rootDir,omitempty"`
			OutDir                           string `json:"outDir,omitempty"`
		}{
			Module:                           "NodeNext",
			ModuleResolution:                 "NodeNext",
			Target:                           "ESNext",
			ForceConsistentCasingInFileNames: true,
			EsModuleInterop:                  true,
			Strict:                           true,
		},
		Include: []string{"src/**/*.*"},
		Exclude: []string{"**/node_modules", "**/.*/"},
	}
}

func generatePackageJSON(projectName string) packageJSON {
	return packageJSON{
		Name:    projectName,
		Version: "1.0.0",
		Type:    "module",
	}
}

func execOutput(args []string, dir string) (string, ExecError) {
	var stderr bytes.Buffer
	var stdout bytes.Buffer

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir

	if err := cmd.Run(); err != nil {
		return "", ExecError{
			stdout: stdout.String(),
			stderr: stderr.String(),
			error:  err,
		}
	}

	return stdout.String(), ExecError{}
}

func copyStaticFile(m *Model, staticFile EmbeddedFile, fileName string) error {
	if err := os.WriteFile(path.Join(m.projectPath, fileName), staticFile, 0644); err != nil {
		return err
	}

	return nil
}

func getProjectPath(projectName string) (string, error) {
	// get the current working directory
	cwd, err := os.Getwd()

	if err != nil {
		return "", err
	}

	// set project path
	projectPath := path.Join(cwd, projectName)

	// check if path exists
	dir, err := os.Open(projectPath)

	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("Unable to create project at %v directory already exists.", projectPath)
	}

	defer dir.Close()

	// create project path
	if err := os.MkdirAll(path.Join(projectPath, "src"), 0755); err != nil {
		return "", err
	}

	return projectPath, nil
}

func writeJson[T packageJSON | tsconfigJSON](dir string, fileName string, data T) error {
	file, err := json.MarshalIndent(data, "", "    ")

	if err != nil {
		return err
	}

	if err := os.WriteFile(path.Join(dir, fileName), file, 0644); err != nil {
		return err
	}

	return nil
}

func installDependency(m *Model, i int) tea.Msg {
	args := []string{"npm", "install"}

	if m.dependencies[i].devDependency {
		args = append(args, "-D")
	}

	args = append(args, m.dependencies[i].name, "--color=always")

	_, err := execOutput(args, m.projectPath)

	if err.error != nil {
		return ErrorMsg{error: err.error}
	}

	return OnInstalledMsg{
		index: i,
	}
}

func runAudit(m *Model) tea.Msg {
	var audit npmAuditJSON

	args := []string{"npm", "audit", "--json"}
	data, err := execOutput(args, m.projectPath)

	// if there are vulenrabilities npm will return as error from stdout
	if err.error != nil {
		jsonErr := json.Unmarshal([]byte(err.stdout), &audit)

		if jsonErr != nil {
			return ErrorMsg{error: err.error}
		} else {
			return AuditMsg{audit: audit}
		}
	}

	jsonErr := json.Unmarshal([]byte(data), &audit)

	if jsonErr != nil {
		return ErrorMsg{error: jsonErr}
	}

	return AuditMsg{audit: audit}
}

func setupProject(m *Model) tea.Msg {
	packageJSON, tsconfigJSON := generatePackageJSON(m.textinput.Value()), generateTsconfigJSON()
	projectPath, err := getProjectPath(m.textinput.Value())

	if err != nil {
		return ErrorMsg{error: err}
	}

	err = writeJson(projectPath, "package.json", packageJSON)
	if err != nil {
		return ErrorMsg{error: err}
	}

	err = writeJson(projectPath, "tsconfig.json", tsconfigJSON)
	if err != nil {
		return ErrorMsg{error: err}
	}

	err = copyStaticFile(m, EslintConfigJs, "eslintrc.cjs")
	if err != nil {
		return ErrorMsg{error: err}
	}

	err = copyStaticFile(m, EslintConfigJs, "esbuild.js")
	if err != nil {
		return ErrorMsg{error: err}
	}

	return SetupMessage{projectPath: projectPath}
}

func extraDependencies(m *Model) tea.Msg {
	var deps []Dependency

	for _, dep := range m.dependencies {
		if !dep.selected {
			continue
		}

		if dep.name == "typescript" {
			err := copyStaticFile(m, IndexTs, "src/index.ts")
			if err != nil {
				return ErrorMsg{error: err}
			}
		}

		if dep.name == "esbuild" {
			err := copyStaticFile(m, EsbuildJs, "esbuild.js")
			if err != nil {
				return ErrorMsg{error: err}
			}
		}

		if dep.name == "react" {
			deps = append(deps,
				Dependency{name: "@types/react", selected: true, devDependency: true},
				Dependency{name: "@types/react-dom", selected: true, devDependency: true},
				Dependency{name: "eslint-plugin-react", selected: true, devDependency: true},
				Dependency{name: "@typescript-eslint/eslint-plugin", selected: true, devDependency: true},
				Dependency{name: "@typescript-eslint/parser", selected: true, devDependency: true},
			)
		}

		if dep.name == "kysely" {
			deps = append(deps,
				Dependency{name: "@types/node", selected: true, devDependency: true},
			)
		}

		if dep.name == "tailwindcss" {
			err := copyStaticFile(m, TailwindConfigJs, "tailwind.config.js")
			if err != nil {
				return ErrorMsg{error: err}
			}
		}
	}

	return ExtraDepsMessage{dependencies: deps}
}

func severityStatus(audit *npmAuditJSON) string {
	if audit.Metadata.Vulnerabilities.Critical > 0 {
		return fmt.Sprintf("%v ", lipgloss.NewStyle().Background(lipgloss.Color(tw.purple800)).Render("Severity: Critical"))
	}

	if audit.Metadata.Vulnerabilities.High > 0 {
		return fmt.Sprintf("%v ", lipgloss.NewStyle().Background(lipgloss.Color(tw.red600)).Render("Severity: High"))
	}

	if audit.Metadata.Vulnerabilities.Moderate > 0 {
		return fmt.Sprintf("%v ", lipgloss.NewStyle().Background(lipgloss.Color(tw.orange400)).Render("Severity: Moderate"))
	}

	if audit.Metadata.Vulnerabilities.Low > 0 {
		return fmt.Sprintf("%v ", lipgloss.NewStyle().Background(lipgloss.Color(tw.amber400)).Render("Severity: Low"))
	}

	if audit.Metadata.Vulnerabilities.Info > 0 {
		return fmt.Sprintf("%v ", lipgloss.NewStyle().Background(lipgloss.Color(tw.neutral600)).Render("Severity: Info"))
	}

	return ""
}
