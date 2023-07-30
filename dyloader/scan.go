// Package dyloader 动态插件加载器
package dyloader

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"
	_ "unsafe"

	control "github.com/FloatTech/zbputils/control"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/ZeroBot-Plugin/dyloader/plugin"
)

var dyType string
var visited bool

//go:linkname matcherList github.com/wdvxdr1123/ZeroBot.matcherList
//go:linkname matcherLock github.com/wdvxdr1123/ZeroBot.matcherLock
//go:linkname defaultEngine github.com/wdvxdr1123/ZeroBot.defaultEngine
var (
	// matcherList 所有主匹配器列表
	matcherList []*zero.Matcher
	// Matcher 修改读写锁
	matcherLock sync.RWMutex
	// defaultEngine zero 的默认 engine
	defaultEngine *zero.Engine
)

var (
	pluginsMu sync.Mutex
	plugins   = make(map[string]*plugin.Plugin)
)

func init() {
	zero.OnCommand("刷新插件", zero.SuperUserPermission).SetBlock(true).FirstPriority().
		Handle(func(ctx *zero.Ctx) {
			err := scan()
			if err != nil {
				ctx.SendChain(message.Text("Error: " + err.Error()))
			} else {
				ctx.SendChain(message.Text("成功!"))
			}
		})
	zero.OnCommand("加载插件", zero.SuperUserPermission).SetBlock(true).FirstPriority().
		Handle(func(ctx *zero.Ctx) {
			model := extension.CommandModel{}
			_ = ctx.Parse(&model)
			_, ok := control.Lookup(model.Args)
			if !ok {
				target := model.Args + dyType
				logrus.Debugln("[dyloader]target:", target)
				path := "plugins/" + target
				err := open(path, target)
				if err != nil {
					ctx.SendChain(message.Text("Error: " + err.Error()))
				} else {
					ctx.SendChain(message.Text("成功!"))
				}
			}
		})
	zero.OnCommand("卸载插件", zero.SuperUserPermission).SetBlock(true).FirstPriority().
		Handle(func(ctx *zero.Ctx) {
			model := extension.CommandModel{}
			_ = ctx.Parse(&model)
			_, ok := control.Lookup(model.Args)
			if ok {
				target := model.Args + dyType
				logrus.Debugln("[dyloader]target:", target)
				pluginsMu.Lock()
				p, ok := plugins[target]
				pluginsMu.Unlock()
				if ok {
					err := plugin.Close(p)
					control.Delete(model.Args)
					pluginsMu.Lock()
					delete(plugins, target)
					pluginsMu.Unlock()
					if err != nil {
						ctx.SendChain(message.Text("Error: " + err.Error()))
					} else {
						ctx.SendChain(message.Text("成功!"))
					}
				} else {
					ctx.SendChain(message.Text("没有这个插件!"))
				}
			}
		})
	go func() {
		time.Sleep(time.Second * 2)
		_ = scan()
	}()
}

func scan() error {
	return filepath.WalkDir("plugins/", load)
}

func open(path, target string) error {
	pluginsMu.Lock()
	_, ok := plugins[target]
	pluginsMu.Unlock()
	if !ok {
		p, err := plugin.Open(path)
		if err == nil {
			var initfunc plugin.Symbol
			initfunc, err = p.Lookup("Inita")
			if err == nil {
				initfunc.(func())()
				logrus.Infoln("[dyloader]加载插件", path, "成功")
				pluginsMu.Lock()
				plugins[target] = p
				pluginsMu.Unlock()
				return nil
			}
			_ = plugin.Close(p)
		}
		if err != nil {
			logrus.Errorln("[dyloader]加载插件", path, "错误:", err)
		}
		return err
	}
	return nil
}

func load(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if d.IsDir() {
		return nil
	}
	n := d.Name()
	if !visited {
		if strings.HasSuffix(n, ".so") {
			dyType = ".so"
			visited = true
		} else if strings.HasSuffix(n, ".dylib") {
			dyType = ".dylib"
			visited = true
		} else if strings.HasSuffix(n, ".dll") {
			dyType = ".dll"
			visited = true
		}
	}
	if strings.HasSuffix(n, ".so") || strings.HasSuffix(n, ".dll") {
		target := path[strings.LastIndex(path, "/")+1:]
		logrus.Debugln("[dyloader]target:", target)
		return open(path, target)
	}
	return nil
}
