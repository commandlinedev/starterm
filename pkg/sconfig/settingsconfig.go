// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package sconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/commandlinedev/starterm/pkg/sconfig/defaultconfig"
	"github.com/commandlinedev/starterm/pkg/starbase"
	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/util/utilfn"
)

const SettingsFile = "settings.json"
const ConnectionsFile = "connections.json"
const ProfilesFile = "profiles.json"

const AnySchema = `
{
  "type": "object",
  "additionalProperties": true
}
`

type AiSettingsType struct {
	AiClear         bool    `json:"ai:*,omitempty"`
	AiPreset        string  `json:"ai:preset,omitempty"`
	AiApiType       string  `json:"ai:apitype,omitempty"`
	AiBaseURL       string  `json:"ai:baseurl,omitempty"`
	AiApiToken      string  `json:"ai:apitoken,omitempty"`
	AiName          string  `json:"ai:name,omitempty"`
	AiModel         string  `json:"ai:model,omitempty"`
	AiOrgID         string  `json:"ai:orgid,omitempty"`
	AIApiVersion    string  `json:"ai:apiversion,omitempty"`
	AiMaxTokens     float64 `json:"ai:maxtokens,omitempty"`
	AiTimeoutMs     float64 `json:"ai:timeoutms,omitempty"`
	AiFontSize      float64 `json:"ai:fontsize,omitempty"`
	AiFixedFontSize float64 `json:"ai:fixedfontsize,omitempty"`
	DisplayName     string  `json:"display:name,omitempty"`
	DisplayOrder    float64 `json:"display:order,omitempty"`
}

type SettingsType struct {
	AppClear                      bool   `json:"app:*,omitempty"`
	AppGlobalHotkey               string `json:"app:globalhotkey,omitempty"`
	AppDismissArchitectureWarning bool   `json:"app:dismissarchitecturewarning,omitempty"`
	AppDefaultNewBlock            string `json:"app:defaultnewblock,omitempty"`

	AiClear         bool    `json:"ai:*,omitempty"`
	AiPreset        string  `json:"ai:preset,omitempty"`
	AiApiType       string  `json:"ai:apitype,omitempty"`
	AiBaseURL       string  `json:"ai:baseurl,omitempty"`
	AiApiToken      string  `json:"ai:apitoken,omitempty"`
	AiName          string  `json:"ai:name,omitempty"`
	AiModel         string  `json:"ai:model,omitempty"`
	AiOrgID         string  `json:"ai:orgid,omitempty"`
	AIApiVersion    string  `json:"ai:apiversion,omitempty"`
	AiMaxTokens     float64 `json:"ai:maxtokens,omitempty"`
	AiTimeoutMs     float64 `json:"ai:timeoutms,omitempty"`
	AiFontSize      float64 `json:"ai:fontsize,omitempty"`
	AiFixedFontSize float64 `json:"ai:fixedfontsize,omitempty"`

	TermClear               bool     `json:"term:*,omitempty"`
	TermFontSize            float64  `json:"term:fontsize,omitempty"`
	TermFontFamily          string   `json:"term:fontfamily,omitempty"`
	TermTheme               string   `json:"term:theme,omitempty"`
	TermDisableWebGl        bool     `json:"term:disablewebgl,omitempty"`
	TermLocalShellPath      string   `json:"term:localshellpath,omitempty"`
	TermLocalShellOpts      []string `json:"term:localshellopts,omitempty"`
	TermScrollback          *int64   `json:"term:scrollback,omitempty"`
	TermCopyOnSelect        *bool    `json:"term:copyonselect,omitempty"`
	TermTransparency        *float64 `json:"term:transparency,omitempty"`
	TermAllowBracketedPaste *bool    `json:"term:allowbracketedpaste,omitempty"`

	EditorMinimapEnabled      bool    `json:"editor:minimapenabled,omitempty"`
	EditorStickyScrollEnabled bool    `json:"editor:stickyscrollenabled,omitempty"`
	EditorWordWrap            bool    `json:"editor:wordwrap,omitempty"`
	EditorFontSize            float64 `json:"editor:fontsize,omitempty"`

	WebClear               bool   `json:"web:*,omitempty"`
	WebOpenLinksInternally bool   `json:"web:openlinksinternally,omitempty"`
	WebDefaultUrl          string `json:"web:defaulturl,omitempty"`
	WebDefaultSearch       string `json:"web:defaultsearch,omitempty"`

	BlockHeaderClear        bool `json:"blockheader:*,omitempty"`
	BlockHeaderShowBlockIds bool `json:"blockheader:showblockids,omitempty"`

	AutoUpdateClear         bool    `json:"autoupdate:*,omitempty"`
	AutoUpdateEnabled       bool    `json:"autoupdate:enabled,omitempty"`
	AutoUpdateIntervalMs    float64 `json:"autoupdate:intervalms,omitempty"`
	AutoUpdateInstallOnQuit bool    `json:"autoupdate:installonquit,omitempty"`
	AutoUpdateChannel       string  `json:"autoupdate:channel,omitempty"`

	MarkdownFontSize      float64 `json:"markdown:fontsize,omitempty"`
	MarkdownFixedFontSize float64 `json:"markdown:fixedfontsize,omitempty"`

	PreviewShowHiddenFiles *bool `json:"preview:showhiddenfiles,omitempty"`

	TabPreset string `json:"tab:preset,omitempty"`

	WidgetClear    bool  `json:"widget:*,omitempty"`
	WidgetShowHelp *bool `json:"widget:showhelp,omitempty"`

	WindowClear                         bool     `json:"window:*,omitempty"`
	WindowTransparent                   bool     `json:"window:transparent,omitempty"`
	WindowBlur                          bool     `json:"window:blur,omitempty"`
	WindowOpacity                       *float64 `json:"window:opacity,omitempty"`
	WindowBgColor                       string   `json:"window:bgcolor,omitempty"`
	WindowReducedMotion                 bool     `json:"window:reducedmotion,omitempty"`
	WindowTileGapSize                   *int64   `json:"window:tilegapsize,omitempty"`
	WindowShowMenuBar                   bool     `json:"window:showmenubar,omitempty"`
	WindowNativeTitleBar                bool     `json:"window:nativetitlebar,omitempty"`
	WindowDisableHardwareAcceleration   bool     `json:"window:disablehardwareacceleration,omitempty"`
	WindowMaxTabCacheSize               int      `json:"window:maxtabcachesize,omitempty"`
	WindowMagnifiedBlockOpacity         *float64 `json:"window:magnifiedblockopacity,omitempty"`
	WindowMagnifiedBlockSize            *float64 `json:"window:magnifiedblocksize,omitempty"`
	WindowMagnifiedBlockBlurPrimaryPx   *int64   `json:"window:magnifiedblockblurprimarypx,omitempty"`
	WindowMagnifiedBlockBlurSecondaryPx *int64   `json:"window:magnifiedblockblursecondarypx,omitempty"`
	WindowConfirmClose                  bool     `json:"window:confirmclose,omitempty"`
	WindowSaveLastWindow                bool     `json:"window:savelastwindow,omitempty"`
	WindowDimensions                    string   `json:"window:dimensions,omitempty"`
	WindowZoom                          *float64 `json:"window:zoom,omitempty"`

	TelemetryClear   bool `json:"telemetry:*,omitempty"`
	TelemetryEnabled bool `json:"telemetry:enabled,omitempty"`

	ConnClear               bool  `json:"conn:*,omitempty"`
	ConnAskBeforeWshInstall *bool `json:"conn:askbeforewshinstall,omitempty"`
	ConnWshEnabled          bool  `json:"conn:wshenabled,omitempty"`
}

type ConfigError struct {
	File string `json:"file"`
	Err  string `json:"err"`
}

type WebBookmark struct {
	Url          string  `json:"url"`
	Title        string  `json:"title,omitempty"`
	Icon         string  `json:"icon,omitempty"`
	IconColor    string  `json:"iconcolor,omitempty"`
	IconUrl      string  `json:"iconurl,omitempty"`
	DisplayOrder float64 `json:"display:order,omitempty"`
}

type FullConfigType struct {
	Settings       SettingsType                   `json:"settings" merge:"meta"`
	MimeTypes      map[string]MimeTypeConfigType  `json:"mimetypes"`
	DefaultWidgets map[string]WidgetConfigType    `json:"defaultwidgets"`
	Widgets        map[string]WidgetConfigType    `json:"widgets"`
	Presets        map[string]starobj.MetaMapType `json:"presets"`
	TermThemes     map[string]TermThemeType       `json:"termthemes"`
	Connections    map[string]ConnKeywords        `json:"connections"`
	Bookmarks      map[string]WebBookmark         `json:"bookmarks"`
	ConfigErrors   []ConfigError                  `json:"configerrors" configfile:"-"`
}
type ConnKeywords struct {
	ConnWshEnabled          *bool  `json:"conn:wshenabled,omitempty"`
	ConnAskBeforeWshInstall *bool  `json:"conn:askbeforewshinstall,omitempty"`
	ConnWshPath             string `json:"conn:wshpath,omitempty"`
	ConnShellPath           string `json:"conn:shellpath,omitempty"`
	ConnIgnoreSshConfig     *bool  `json:"conn:ignoresshconfig,omitempty"`

	DisplayHidden *bool   `json:"display:hidden,omitempty"`
	DisplayOrder  float32 `json:"display:order,omitempty"`

	TermClear      bool    `json:"term:*,omitempty"`
	TermFontSize   float64 `json:"term:fontsize,omitempty"`
	TermFontFamily string  `json:"term:fontfamily,omitempty"`
	TermTheme      string  `json:"term:theme,omitempty"`

	CmdEnv            map[string]string `json:"cmd:env,omitempty"`
	CmdInitScript     string            `json:"cmd:initscript,omitempty"`
	CmdInitScriptSh   string            `json:"cmd:initscript.sh,omitempty"`
	CmdInitScriptBash string            `json:"cmd:initscript.bash,omitempty"`
	CmdInitScriptZsh  string            `json:"cmd:initscript.zsh,omitempty"`
	CmdInitScriptPwsh string            `json:"cmd:initscript.pwsh,omitempty"`
	CmdInitScriptFish string            `json:"cmd:initscript.fish,omitempty"`

	SshUser                         *string  `json:"ssh:user,omitempty"`
	SshHostName                     *string  `json:"ssh:hostname,omitempty"`
	SshPort                         *string  `json:"ssh:port,omitempty"`
	SshIdentityFile                 []string `json:"ssh:identityfile,omitempty"`
	SshBatchMode                    *bool    `json:"ssh:batchmode,omitempty"`
	SshPubkeyAuthentication         *bool    `json:"ssh:pubkeyauthentication,omitempty"`
	SshPasswordAuthentication       *bool    `json:"ssh:passwordauthentication,omitempty"`
	SshKbdInteractiveAuthentication *bool    `json:"ssh:kbdinteractiveauthentication,omitempty"`
	SshPreferredAuthentications     []string `json:"ssh:preferredauthentications,omitempty"`
	SshAddKeysToAgent               *bool    `json:"ssh:addkeystoagent,omitempty"`
	SshIdentityAgent                *string  `json:"ssh:identityagent,omitempty"`
	SshIdentitiesOnly               *bool    `json:"ssh:identitiesonly,omitempty"`
	SshProxyJump                    []string `json:"ssh:proxyjump,omitempty"`
	SshUserKnownHostsFile           []string `json:"ssh:userknownhostsfile,omitempty"`
	SshGlobalKnownHostsFile         []string `json:"ssh:globalknownhostsfile,omitempty"`
}

func DefaultBoolPtr(arg *bool, def bool) bool {
	if arg == nil {
		return def
	}
	return *arg
}

func goBackWS(barr []byte, offset int) int {
	if offset >= len(barr) {
		offset = offset - 1
	}
	for i := offset - 1; i >= 0; i-- {
		if barr[i] == ' ' || barr[i] == '\t' || barr[i] == '\n' || barr[i] == '\r' {
			continue
		}
		return i
	}
	return 0
}

func isTrailingCommaError(barr []byte, offset int) bool {
	if offset >= len(barr) {
		offset = offset - 1
	}
	offset = goBackWS(barr, offset)
	if barr[offset] == '}' {
		offset = goBackWS(barr, offset)
		if barr[offset] == ',' {
			return true
		}
	}
	return false
}

func readConfigHelper(fileName string, barr []byte, readErr error) (starobj.MetaMapType, []ConfigError) {
	var cerrs []ConfigError
	if readErr != nil && !os.IsNotExist(readErr) {
		cerrs = append(cerrs, ConfigError{File: fileName, Err: readErr.Error()})
	}
	if len(barr) == 0 {
		return nil, cerrs
	}
	var rtn starobj.MetaMapType
	err := json.Unmarshal(barr, &rtn)
	if err != nil {
		if syntaxErr, ok := err.(*json.SyntaxError); ok {
			offset := syntaxErr.Offset
			if offset > 0 {
				offset = offset - 1
			}
			lineNum, colNum := utilfn.GetLineColFromOffset(barr, int(offset))
			isTrailingComma := isTrailingCommaError(barr, int(offset))
			if isTrailingComma {
				err = fmt.Errorf("json syntax error at line %d, col %d: probably an extra trailing comma: %v", lineNum, colNum, syntaxErr)
			} else {
				err = fmt.Errorf("json syntax error at line %d, col %d: %v", lineNum, colNum, syntaxErr)
			}
		}
		cerrs = append(cerrs, ConfigError{File: fileName, Err: err.Error()})
	}
	return rtn, cerrs
}

func readConfigFileFS(fsys fs.FS, logPrefix string, fileName string) (starobj.MetaMapType, []ConfigError) {
	barr, readErr := fs.ReadFile(fsys, fileName)
	if readErr != nil {
		// If we get an error, we may be using the wrong path separator for the given FS interface. Try switching the separator.
		barr, readErr = fs.ReadFile(fsys, filepath.ToSlash(fileName))
	}
	return readConfigHelper(logPrefix+fileName, barr, readErr)
}

func ReadDefaultsConfigFile(fileName string) (starobj.MetaMapType, []ConfigError) {
	return readConfigFileFS(defaultconfig.ConfigFS, "defaults:", fileName)
}

func ReadStarHomeConfigFile(fileName string) (starobj.MetaMapType, []ConfigError) {
	configDirAbsPath := starbase.GetStarConfigDir()
	configDirFsys := os.DirFS(configDirAbsPath)
	return readConfigFileFS(configDirFsys, "", fileName)
}

func WriteStarHomeConfigFile(fileName string, m starobj.MetaMapType) error {
	configDirAbsPath := starbase.GetStarConfigDir()
	fullFileName := filepath.Join(configDirAbsPath, fileName)
	barr, err := jsonMarshalConfigInOrder(m)
	if err != nil {
		return err
	}
	return os.WriteFile(fullFileName, barr, 0644)
}

// simple merge that overwrites
func mergeMetaMapSimple(m starobj.MetaMapType, toMerge starobj.MetaMapType) starobj.MetaMapType {
	if m == nil {
		return toMerge
	}
	if toMerge == nil {
		return m
	}
	for k, v := range toMerge {
		if v == nil {
			delete(m, k)
			continue
		}
		m[k] = v
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func mergeMetaMap(m starobj.MetaMapType, toMerge starobj.MetaMapType, simpleMerge bool) starobj.MetaMapType {
	if simpleMerge {
		return mergeMetaMapSimple(m, toMerge)
	} else {
		return starobj.MergeMeta(m, toMerge, true)
	}
}

func selectDirEntsBySuffix(dirEnts []fs.DirEntry, fileNameSuffix string) []fs.DirEntry {
	var rtn []fs.DirEntry
	for _, ent := range dirEnts {
		if ent.IsDir() {
			continue
		}
		if !strings.HasSuffix(ent.Name(), fileNameSuffix) {
			continue
		}
		rtn = append(rtn, ent)
	}
	return rtn
}

func SortFileNameDescend(files []fs.DirEntry) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() > files[j].Name()
	})
}

// Read and merge all files in the specified directory matching the supplied suffix
func readConfigFilesForDir(fsys fs.FS, logPrefix string, dirName string, fileName string, simpleMerge bool) (starobj.MetaMapType, []ConfigError) {
	dirEnts, _ := fs.ReadDir(fsys, dirName)
	suffixEnts := selectDirEntsBySuffix(dirEnts, fileName+".json")
	SortFileNameDescend(suffixEnts)
	var rtn starobj.MetaMapType
	var errs []ConfigError
	for _, ent := range suffixEnts {
		fileVal, cerrs := readConfigFileFS(fsys, logPrefix, filepath.Join(dirName, ent.Name()))
		rtn = mergeMetaMap(rtn, fileVal, simpleMerge)
		errs = append(errs, cerrs...)
	}
	return rtn, errs
}

// Read and merge all files in the specified config filesystem matching the patterns `<partName>.json` and `<partName>/*.json`
func readConfigPartForFS(fsys fs.FS, logPrefix string, partName string, simpleMerge bool) (starobj.MetaMapType, []ConfigError) {
	config, errs := readConfigFilesForDir(fsys, logPrefix, partName, "", simpleMerge)
	allErrs := errs
	rtn := config
	config, errs = readConfigFileFS(fsys, logPrefix, partName+".json")
	allErrs = append(allErrs, errs...)
	return mergeMetaMap(rtn, config, simpleMerge), allErrs
}

// Combine files from the defaults and home directory for the specified config part name
func readConfigPart(partName string, simpleMerge bool) (starobj.MetaMapType, []ConfigError) {
	configDirAbsPath := starbase.GetStarConfigDir()
	configDirFsys := os.DirFS(configDirAbsPath)
	defaultConfigs, cerrs := readConfigPartForFS(defaultconfig.ConfigFS, "defaults:", partName, simpleMerge)
	homeConfigs, cerrs1 := readConfigPartForFS(configDirFsys, "", partName, simpleMerge)

	rtn := defaultConfigs
	allErrs := append(cerrs, cerrs1...)
	return mergeMetaMap(rtn, homeConfigs, simpleMerge), allErrs
}

// this function should only be called by the sconfig code.
// in golang code, the best way to get the current config is via the watcher -- sconfig.GetWatcher().GetFullConfig()
func ReadFullConfig() FullConfigType {
	var fullConfig FullConfigType
	configRType := reflect.TypeOf(fullConfig)
	configRVal := reflect.ValueOf(&fullConfig).Elem()
	for fieldIdx := 0; fieldIdx < configRType.NumField(); fieldIdx++ {
		field := configRType.Field(fieldIdx)
		if field.PkgPath != "" {
			continue
		}
		configFile := field.Tag.Get("configfile")
		if configFile == "-" {
			continue
		}
		jsonTag := utilfn.GetJsonTag(field)
		simpleMerge := field.Tag.Get("merge") == ""
		var configPart starobj.MetaMapType
		var errs []ConfigError
		if jsonTag == "-" || jsonTag == "" {
			continue
		} else {
			configPart, errs = readConfigPart(jsonTag, simpleMerge)
		}
		fullConfig.ConfigErrors = append(fullConfig.ConfigErrors, errs...)
		if configPart != nil {
			fieldPtr := configRVal.Field(fieldIdx).Addr().Interface()
			utilfn.ReUnmarshal(fieldPtr, configPart)
		}
	}
	return fullConfig
}

func GetConfigSubdirs() []string {
	var fullConfig FullConfigType
	configRType := reflect.TypeOf(fullConfig)
	var retVal []string
	configDirAbsPath := starbase.GetStarConfigDir()
	for fieldIdx := 0; fieldIdx < configRType.NumField(); fieldIdx++ {
		field := configRType.Field(fieldIdx)
		if field.PkgPath != "" {
			continue
		}
		configFile := field.Tag.Get("configfile")
		if configFile == "-" {
			continue
		}
		jsonTag := utilfn.GetJsonTag(field)
		if jsonTag != "-" && jsonTag != "" && jsonTag != "settings" {
			retVal = append(retVal, filepath.Join(configDirAbsPath, jsonTag))
		}
	}
	log.Printf("subdirs: %v\n", retVal)
	return retVal
}

func getConfigKeyType(configKey string) reflect.Type {
	ctype := reflect.TypeOf(SettingsType{})
	for i := 0; i < ctype.NumField(); i++ {
		field := ctype.Field(i)
		jsonTag := utilfn.GetJsonTag(field)
		if jsonTag == configKey {
			return field.Type
		}
	}
	return nil
}

func getConfigKeyNamespace(key string) string {
	colonIdx := strings.Index(key, ":")
	if colonIdx == -1 {
		return ""
	}
	return key[:colonIdx]
}

func orderConfigKeys(m starobj.MetaMapType) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		k1 := keys[i]
		k2 := keys[j]
		k1ns := getConfigKeyNamespace(k1)
		k2ns := getConfigKeyNamespace(k2)
		if k1ns != k2ns {
			return k1ns < k2ns
		}
		return k1 < k2
	})
	return keys
}

func reindentJson(barr []byte, indentStr string) []byte {
	if len(barr) < 2 {
		return barr
	}
	if barr[0] != '{' && barr[0] != '[' {
		return barr
	}
	if !bytes.Contains(barr, []byte("\n")) {
		return barr
	}
	outputLines := bytes.Split(barr, []byte("\n"))
	for i, line := range outputLines {
		if i == 0 {
			continue
		}
		outputLines[i] = append([]byte(indentStr), line...)
	}
	return bytes.Join(outputLines, []byte("\n"))
}

func jsonMarshalConfigInOrder(m starobj.MetaMapType) ([]byte, error) {
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	var buf bytes.Buffer
	orderedKeys := orderConfigKeys(m)
	buf.WriteString("{\n")
	for idx, key := range orderedKeys {
		val := m[key]
		keyBarr, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		valBarr, err := json.MarshalIndent(val, "", "  ")
		if err != nil {
			return nil, err
		}
		valBarr = reindentJson(valBarr, "  ")
		buf.WriteString("  ")
		buf.Write(keyBarr)
		buf.WriteString(": ")
		buf.Write(valBarr)
		if idx < len(orderedKeys)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("}")
	return buf.Bytes(), nil
}

var dummyNumber json.Number

func convertJsonNumber(num json.Number, ctype reflect.Type) (interface{}, error) {
	// ctype might be int64, float64, string, *int64, *float64, *string
	// switch on ctype first
	if ctype.Kind() == reflect.Pointer {
		ctype = ctype.Elem()
	}
	if reflect.Int64 == ctype.Kind() {
		if ival, err := num.Int64(); err == nil {
			return ival, nil
		}
		return nil, fmt.Errorf("invalid number for int64: %s", num)
	}
	if reflect.Float64 == ctype.Kind() {
		if fval, err := num.Float64(); err == nil {
			return fval, nil
		}
		return nil, fmt.Errorf("invalid number for float64: %s", num)
	}
	if reflect.String == ctype.Kind() {
		return num.String(), nil
	}
	return nil, fmt.Errorf("cannot convert number to %s", ctype)
}

func SetBaseConfigValue(toMerge starobj.MetaMapType) error {
	m, cerrs := ReadStarHomeConfigFile(SettingsFile)
	if len(cerrs) > 0 {
		return fmt.Errorf("error reading config file: %v", cerrs[0])
	}
	if m == nil {
		m = make(starobj.MetaMapType)
	}
	for configKey, val := range toMerge {
		ctype := getConfigKeyType(configKey)
		if ctype == nil {
			return fmt.Errorf("invalid config key: %s", configKey)
		}
		if val == nil {
			delete(m, configKey)
		} else {
			rtype := reflect.TypeOf(val)
			if rtype == reflect.TypeOf(dummyNumber) {
				convertedVal, err := convertJsonNumber(val.(json.Number), ctype)
				if err != nil {
					return fmt.Errorf("cannot convert %s: %v", configKey, err)
				}
				val = convertedVal
				rtype = reflect.TypeOf(val)
			}
			if rtype != ctype {
				if ctype == reflect.PointerTo(rtype) {
					m[configKey] = &val
				} else {
					return fmt.Errorf("invalid value type for %s: %T", configKey, val)
				}
			}
			m[configKey] = val
		}
	}
	return WriteStarHomeConfigFile(SettingsFile, m)
}

func SetConnectionsConfigValue(connName string, toMerge starobj.MetaMapType) error {
	m, cerrs := ReadStarHomeConfigFile(ConnectionsFile)
	if len(cerrs) > 0 {
		return fmt.Errorf("error reading config file: %v", cerrs[0])
	}
	if m == nil {
		m = make(starobj.MetaMapType)
	}
	connData := m.GetMap(connName)
	if connData == nil {
		connData = make(starobj.MetaMapType)
	}
	for configKey, val := range toMerge {
		connData[configKey] = val
	}
	m[connName] = connData
	return WriteStarHomeConfigFile(ConnectionsFile, m)
}

type WidgetConfigType struct {
	DisplayOrder  float64          `json:"display:order,omitempty"`
	DisplayHidden bool             `json:"display:hidden,omitempty"`
	Icon          string           `json:"icon,omitempty"`
	Color         string           `json:"color,omitempty"`
	Label         string           `json:"label,omitempty"`
	Description   string           `json:"description,omitempty"`
	Magnified     bool             `json:"magnified,omitempty"`
	BlockDef      starobj.BlockDef `json:"blockdef"`
}

type MimeTypeConfigType struct {
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

type TermThemeType struct {
	DisplayName         string  `json:"display:name"`
	DisplayOrder        float64 `json:"display:order"`
	Black               string  `json:"black"`
	Red                 string  `json:"red"`
	Green               string  `json:"green"`
	Yellow              string  `json:"yellow"`
	Blue                string  `json:"blue"`
	Magenta             string  `json:"magenta"`
	Cyan                string  `json:"cyan"`
	White               string  `json:"white"`
	BrightBlack         string  `json:"brightBlack"`
	BrightRed           string  `json:"brightRed"`
	BrightGreen         string  `json:"brightGreen"`
	BrightYellow        string  `json:"brightYellow"`
	BrightBlue          string  `json:"brightBlue"`
	BrightMagenta       string  `json:"brightMagenta"`
	BrightCyan          string  `json:"brightCyan"`
	BrightWhite         string  `json:"brightWhite"`
	Gray                string  `json:"gray"`
	CmdText             string  `json:"cmdtext"`
	Foreground          string  `json:"foreground"`
	SelectionBackground string  `json:"selectionBackground"`
	Background          string  `json:"background"`
	Cursor              string  `json:"cursor"`
}
