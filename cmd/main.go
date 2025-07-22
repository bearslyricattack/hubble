package main

import (
	"context"
	"flag"
	"hubble/config"
	"hubble/datastore"
	"hubble/hubble"
	"hubble/server"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {
	// 命令行参数只保留配置文件路径
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 设置Redis客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// 测试Redis连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("连接Redis失败: %v", err)
	}
	log.Println("成功连接到Redis")

	// 创建数据存储
	dataStore := datastore.NewDataStore(redisClient)

	// 创建HTTP服务器
	server := server.NewServer(dataStore)

	// 启动Hubble流量收集器
	collector := hubble.NewCollector(cfg.Hubble.Addr, dataStore)
	go collector.Start()

	// 启动HTTP服务器
	go func() {
		log.Printf("在%s地址启动HTTP服务器", cfg.HTTP.Addr)
		if err := http.ListenAndServe(cfg.HTTP.Addr, server.Router); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP服务器错误: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("正在关闭服务...")
	collector.Stop()
}
