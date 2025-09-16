package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/andreykaipov/goobs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/andreykaipov/goobs/api/typedefs"
	"golang.design/x/hotkey"
	"gopkg.in/yaml.v3"
)

// Config is the struct of YAML configuration file
type Config struct {
	OBS struct {
		Address  string `yaml:"address"`
		Port     string `yaml:"port"`
		Password string `yaml:"password"`
	} `yaml:"obs"`
	Masks []MaskConfig `yaml:"masks"`
}

// MaskConfig represents a single mask configuration
type MaskConfig struct {
	Name   string `yaml:"name"`
	Source string `yaml:"source"`
	Scene  string `yaml:"scene"` // Optional: scene name for scene items, default empty " ", auto detect
	Hotkey struct {
		Key       string   `yaml:"key"`
		Modifiers []string `yaml:"modifiers"`
	} `yaml:"hotkey"`
}

// OBSClient wraps the OBS WebSocket client and related data
type OBSClient struct {
	client *goobs.Client
	mutex  sync.Mutex
}

// MaskState tracks the state of each mask
type MaskState struct {
	Name   string
	Source string
	Scene  string
	Active bool
}

func main() {
	// Define command-line flag for config file path
	configFile := flag.String("f", "", "Path to the configuration file (optional, defaults to conf.yml)")
	flag.Parse()

	var configPath string
	if *configFile != "" {
		configPath = *configFile
	} else {
		configPath = "conf.yaml"
		log.Printf("No -f specified, using default config: %s", configPath)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("Config file %s does not exist", configPath)
	}

	// Load configure
	configData, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read config file %s: %v", configPath, err)
	}
	var cfg Config
	err = yaml.Unmarshal(configData, &cfg)
	if err != nil {
		log.Fatalf("Failed to parse config file %s: %v", configPath, err)
	}

	// Clean address to ensure no protocol or extra colons
	// W : IPv6 unsportted !!!!!!!
	address := strings.TrimPrefix(strings.TrimPrefix(cfg.OBS.Address, "http://"), "ws://")
	if strings.Contains(address, ":") {
		log.Fatalf("Invalid address format in config: %s (must not contain ':' or protocol)", cfg.OBS.Address)
	}
	addr := fmt.Sprintf("%s:%s", address, cfg.OBS.Port)
	log.Printf("Connecting to OBS WebSocket at: %s", addr)

	client, err := goobs.New(
		addr,
		goobs.WithPassword(cfg.OBS.Password),
	)
	if err != nil {
		log.Fatalf("Failed to create OBS client: %v", err)
	}
	if client == nil {
		log.Fatalf("OBS client is nil, initialization failed")
	}
	obsClient := &OBSClient{
		client: client,
	}

	// Initialize mask states and sync initial visibility from OBS server
	maskStates := make(map[string]*MaskState)
	for _, mask := range cfg.Masks {
		if mask.Name == "" || mask.Source == "" {
			log.Printf("Invalid mask configuration: name=%s, source=%s, skipping", mask.Name, mask.Source)
			continue
		}
		state := &MaskState{
			Name:   mask.Name,
			Source: mask.Source,
			Scene:  mask.Scene,
			Active: false, // Default
		}

		sceneName := mask.Scene
		if sceneName == "" {
			// Get all scenes and find the one containing the source
			resp, err := client.Scenes.GetSceneList()
			if err != nil {
				log.Printf("Failed to get scene list for mask %s: %v, assuming hidden", mask.Name, err)
				continue
			}
			if len(resp.Scenes) == 0 {
				log.Printf("No scenes found in OBS for mask %s, assuming hidden", mask.Name)
				continue
			}
			log.Println("Available scenes:")
			for _, v := range resp.Scenes {
				fmt.Printf("%2d %s\n", v.SceneIndex, v.SceneName)
			}
			// Try current scene first
			currentScene, err := client.Scenes.GetCurrentProgramScene()
			if err != nil {
				log.Printf("Failed to get current scene for mask %s: %v", mask.Name, err)
			} else if currentScene.CurrentProgramSceneName != "" {
				currentSceneName := currentScene.CurrentProgramSceneName
				params := sceneitems.GetSceneItemListParams{
					SceneName: &currentSceneName,
				}
				respItems, err := client.SceneItems.GetSceneItemList(&params)
				if err != nil {
					log.Printf("Failed to get scene items for current scene %s for mask %s: %v", currentSceneName, mask.Name, err)
				} else {
					itemID := findSceneItemID(respItems.SceneItems, mask.Source)
					if itemID != 0 {
						sceneName = currentSceneName
						log.Printf("Found mask %s in current scene: %s", mask.Name, sceneName)
					}
				}
			}
			// If not in current scene, search all scenes
			if sceneName == "" {
				for _, scene := range resp.Scenes {
					if scene.SceneName == "" {
						continue
					}
					params := sceneitems.GetSceneItemListParams{
						SceneName: &scene.SceneName,
					}
					respItems, err := client.SceneItems.GetSceneItemList(&params)
					if err != nil {
						log.Printf("Failed to get scene items for scene %s for mask %s: %v", scene.SceneName, mask.Name, err)
						continue
					}
					itemID := findSceneItemID(respItems.SceneItems, mask.Source)
					if itemID != 0 {
						sceneName = scene.SceneName
						log.Printf("Found mask %s in scene: %s", mask.Name, sceneName)
						break
					}
				}
			}
			if sceneName == "" {
				log.Printf("No scene found containing mask %s (source: %s), assuming hidden", mask.Name, mask.Source)
				continue
			}
			state.Scene = sceneName
		} else if sceneName == "场景" || strings.TrimSpace(sceneName) == "" {
			log.Printf("Invalid scene name '%s' for mask %s, assuming hidden", sceneName, mask.Name)
			continue
		}

		params := sceneitems.GetSceneItemListParams{
			SceneName: &sceneName,
		}
		respItems, err := client.SceneItems.GetSceneItemList(&params)
		if err != nil {
			log.Printf("Failed to get scene items for mask %s (scene %s): %v", mask.Name, sceneName, err)
			continue
		}
		itemID := int(findSceneItemID(respItems.SceneItems, mask.Source))
		if itemID == 0 {
			log.Printf("Scene item %s not found in scene %s for mask %s, assuming hidden", mask.Source, sceneName, mask.Name)
			continue
		}
		// Get enabled status
		enabledParams := sceneitems.GetSceneItemEnabledParams{
			SceneName:   &sceneName,
			SceneItemId: &itemID,
		}
		enabledResp, err := client.SceneItems.GetSceneItemEnabled(&enabledParams)
		if err != nil {
			log.Printf("Failed to get enabled status for mask %s (scene %s, itemID %d): %v", mask.Name, sceneName, itemID, err)
			continue
		}
		state.Active = enabledResp.SceneItemEnabled
		log.Printf("Synced mask %s (scene item in %s) initial state: %v", mask.Name, sceneName, state.Active)
		maskStates[mask.Name] = state
	}

	// Register hotkeys
	for _, mask := range cfg.Masks {
		if mask.Name == "" {
			log.Printf("Skipping hotkey registration for mask with empty name")
			continue
		}

		if state, exists := maskStates[mask.Name]; !exists || state.Scene == "" {
			log.Printf("Mask state for %s not found or invalid scene, skipping hotkey registration", mask.Name)
			continue
		}
		// Convert modifiers to hotkey.Modifier
		var mods []hotkey.Modifier
		for _, mod := range mask.Hotkey.Modifiers {
			m := strings.ToLower(mod)
			switch m {
			case "ctrl":
				mods = append(mods, hotkey.ModCtrl)
			case "shift":
				mods = append(mods, hotkey.ModShift)
			case "alt":
				mods = append(mods, hotkey.ModAlt)
			case "win":
				mods = append(mods, hotkey.ModWin) // Windows key
			default:
				log.Printf("Unsupported modifier %s for mask %s, skipping", mod, mask.Name)
				continue
			}
		}

		// Map key string to hotkey.Key
		var key hotkey.Key
		switch strings.ToUpper(mask.Hotkey.Key) {
		case "M":
			key = hotkey.KeyM
		case "E":
			key = hotkey.KeyE
		case "A":
			key = hotkey.KeyA
		case "B":
			key = hotkey.KeyB
		case "C":
			key = hotkey.KeyC
		case "D":
			key = hotkey.KeyD
		case "F":
			key = hotkey.KeyF
		case "G":
			key = hotkey.KeyG
		case "H":
			key = hotkey.KeyH
		case "I":
			key = hotkey.KeyI
		case "J":
			key = hotkey.KeyJ
		case "K":
			key = hotkey.KeyK
		case "L":
			key = hotkey.KeyL
		case "N":
			key = hotkey.KeyN
		case "O":
			key = hotkey.KeyO
		case "P":
			key = hotkey.KeyP
		case "Q":
			key = hotkey.KeyQ
		case "R":
			key = hotkey.KeyR
		case "S":
			key = hotkey.KeyS
		case "T":
			key = hotkey.KeyT
		case "U":
			key = hotkey.KeyU
		case "V":
			key = hotkey.KeyV
		case "W":
			key = hotkey.KeyW
		case "X":
			key = hotkey.KeyX
		case "Y":
			key = hotkey.KeyY
		case "Z":
			key = hotkey.KeyZ
		default:
			log.Printf("Unsupported key %s for mask %s, skipping", mask.Hotkey.Key, mask.Name)
			continue
		}

		hk := hotkey.New(mods, key)
		err := hk.Register()
		if err != nil {
			log.Printf("Failed to register hotkey for mask %s: %v", mask.Name, err)
			continue
		}

		// Listen for keydown in goroutine
		go func(name string, state *MaskState, source string, scene string, hk *hotkey.Hotkey) {
			for {
				select {
				case <-hk.Keydown(): // Block
					log.Printf("Hotkey triggered for mask %s", name)
					obsClient.toggleMask(state, source, scene)
				}
			}
		}(mask.Name, maskStates[mask.Name], mask.Source, mask.Scene, hk)

		log.Printf("Registered hotkey %s+%s for mask %s", strings.Join(mask.Hotkey.Modifiers, "+"), mask.Hotkey.Key, mask.Name)
	}

	log.Println("Hotkey listeners started. Press hotkeys to toggle masks...")
	log.Println("To exit, press Ctrl+C")

	// wait sig
	select {}
}

// findSceneItemID retrieves the scene item ID for a given source from scene items list
func findSceneItemID(sceneItems []*typedefs.SceneItem, sourceName string) int64 {
	for _, item := range sceneItems {
		if item.SourceName == sourceName {
			return int64(item.SceneItemID)
		}
	}
	return 0
}

// getSceneItemEnabled retrieves the enabled status for a scene item
func getSceneItemEnabled(client *goobs.Client, sceneName string, itemID int64) (bool, error) {
	itemIDc := int(itemID)
	params := sceneitems.GetSceneItemEnabledParams{
		SceneName:   &sceneName,
		SceneItemId: &itemIDc,
	}
	resp, err := client.SceneItems.GetSceneItemEnabled(&params)
	if err != nil {
		return false, err
	}
	return resp.SceneItemEnabled, nil
}

// toggleMask toggles the visibility of a mask in OBS
func (c *OBSClient) toggleMask(state *MaskState, sourceName, sceneName string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.client == nil {
		log.Printf("OBS client is nil, cannot toggle mask %s", state.Name)
		return
	}
	if state == nil {
		log.Printf("Mask state is nil for source %s, cannot toggle", sourceName)
		return
	}

	state.Active = !state.Active
	visible := state.Active
	log.Printf("Attempting to toggle mask %s to %v", state.Name, visible)

	if sceneName == "" {
		resp, err := c.client.Scenes.GetSceneList()
		if err != nil {
			log.Printf("Failed to get scene list for mask %s: %v", state.Name, err)
			return
		}
		if len(resp.Scenes) == 0 {
			log.Printf("No scenes found in OBS for mask %s", state.Name)
			return
		}
		for _, scene := range resp.Scenes {
			if scene.SceneName == "" {
				log.Printf("Skipping empty scene name for mask %s", state.Name)
				continue
			}
			params := sceneitems.GetSceneItemListParams{
				SceneName: &scene.SceneName,
			}
			respItems, err := c.client.SceneItems.GetSceneItemList(&params)
			if err != nil {
				log.Printf("Failed to get scene items for scene %s : %v", scene.SceneName, err)
				continue
			}
			itemID := findSceneItemID(respItems.SceneItems, sourceName)
			if itemID != 0 {
				sceneName = scene.SceneName
				log.Printf("Found mask %s in scene: %s", state.Name, sceneName)
				break
			}
		}
		if sceneName == "" {
			currentScene, err := c.client.Scenes.GetCurrentProgramScene()
			if err != nil {
				log.Printf("Failed to get current scene for mask %s: %v", state.Name, err)
				return
			}
			if currentScene.CurrentProgramSceneName == "" {
				log.Printf("Current scene name is empty for mask %s", state.Name)
				return
			}
			sceneName = currentScene.CurrentProgramSceneName
			log.Printf("No scene specified for mask %s, using current scene: %s", state.Name, sceneName)
		}
		state.Scene = sceneName
	}

	// Handle as scene item
	params := sceneitems.GetSceneItemListParams{
		SceneName: &sceneName,
	}
	respItems, err := c.client.SceneItems.GetSceneItemList(&params)
	if err != nil {
		log.Printf("Failed to get scene items for mask %s (scene %s): %v", state.Name, sceneName, err)
		return
	}
	itemID := findSceneItemID(respItems.SceneItems, sourceName)
	if itemID == 0 {
		log.Printf("Failed to toggle mask %s: scene item %s not found in scene %s", state.Name, sourceName, sceneName)
		return
	}
	// Toggle enabled status
	itemIDc := int(itemID)
	enabledParams := sceneitems.SetSceneItemEnabledParams{
		SceneName:        &sceneName,
		SceneItemId:      &itemIDc,
		SceneItemEnabled: &visible,
	}
	_, err = c.client.SceneItems.SetSceneItemEnabled(&enabledParams)
	if err != nil {
		log.Printf("Failed to toggle mask %s (scene item in %s): %v", state.Name, sceneName, err)
		return
	}
	log.Printf("Mask %s (scene item in %s) toggled to %v", state.Name, sceneName, visible)
}
