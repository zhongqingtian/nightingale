package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/didi/nightingale/src/common/identity"
	"github.com/didi/nightingale/src/common/loggeri"
	"github.com/didi/nightingale/src/common/report"
	"github.com/didi/nightingale/src/modules/index/cache"
	"github.com/didi/nightingale/src/modules/index/config"
	"github.com/didi/nightingale/src/modules/index/http/routes"
	"github.com/didi/nightingale/src/modules/index/rpc"
	"github.com/didi/nightingale/src/toolkits/http"
	"github.com/didi/nightingale/src/toolkits/stats"

	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/file"
	"github.com/toolkits/pkg/logger"
	"github.com/toolkits/pkg/runner"
)

var (
	vers *bool
	help *bool
	conf *string

	version = "No Version Provided"
)

func init() {
	vers = flag.Bool("v", false, "display the version.")
	help = flag.Bool("h", false, "print this help.")
	conf = flag.String("f", "", "specify configuration file.")
	flag.Parse()

	if *vers { // 取地址值
		fmt.Println("Version:", version)
		os.Exit(0)
	}

	if *help {
		flag.Usage()
		os.Exit(0)
	}
}

func main() {
	aconf() // 判断声明配置文件
	pconf() // 解析配置文件
	start()

	cfg := config.Config

	loggeri.Init(cfg.Logger)   // 初始化日志配置
	go stats.Init("n9e.index") // 初始化配置，定时推送 n9e.index前缀的数据到指定url接口

	identity.Parse() // 解析脚本配置
	cache.InitDB(cfg.Cache)

	go report.Init(cfg.Report, "rdb") // 初始化连接上报db数据库
	go rpc.Start() // 启动 rpc 三个接口监听

	r := gin.New() // http gin框架服务　启动
	routes.Config(r)
	http.Start(r, "index", cfg.Logger.Level)
	ending() // 阻塞主进程，处理优雅退出
}

// auto detect configuration file
func aconf() {
	if *conf != "" && file.IsExist(*conf) {
		return
	}

	*conf = "etc/index.local.yml"
	if file.IsExist(*conf) {
		return
	}

	*conf = "etc/index.yml"
	if file.IsExist(*conf) {
		return
	}

	fmt.Println("no configuration file for index")
	os.Exit(1)
}

// parse configuration file
func pconf() {
	if err := config.Parse(*conf); err != nil {
		fmt.Println("cannot parse configuration file:", err)
		os.Exit(1)
	}
}

func ending() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	select {
	case <-c:
		fmt.Printf("stop signal caught, stopping... pid=%d\n", os.Getpid())
	}

	logger.Close()
	http.Shutdown() // 关闭连接资源
	fmt.Println("sender stopped successfully")
}

func start() {
	runner.Init() // 设置一下默认启动环境配置
	fmt.Println("index start, use configuration file:", *conf)
	fmt.Println("runner.Cwd:", runner.Cwd)
	fmt.Println("runner.Hostname:", runner.Hostname)
}
