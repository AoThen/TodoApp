.PHONY: help build run stop clean logs test

# 默认目标
help:
	@echo "TodoApp Docker 管理命令"
	@echo ""
	@echo "使用方法:"
	@echo "  make build      - 构建 Docker 镜像"
	@echo "  make run        - 启动容器"
	@echo "  make stop       - 停止容器"
	@echo "  make restart    - 重启容器"
	@echo "  make logs       - 查看容器日志"
	@echo "  make clean      - 清理容器和镜像"
	@echo "  make test       - 运行测试"
	@echo "  make shell      - 进入容器 shell"
	@echo "  make db-backup  - 备份数据库"
	@echo "  make db-restore - 恢复数据库"

# 构建 Docker 镜像
build:
	@echo "🔨 构建 Docker 镜像..."
	docker build -t todoapp:latest .
	@echo "✅ 构建完成！"
	@docker images todoapp:latest

# 启动容器
run:
	@echo "🚀 启动容器..."
	docker-compose up -d
	@echo "✅ 容器已启动"
	@echo "📊 查看状态: make logs"
	@echo "🌐 访问应用: http://localhost:8080"

# 停止容器
stop:
	@echo "⏹️  停止容器..."
	docker-compose down
	@echo "✅ 容器已停止"

# 重启容器
restart:
	@echo "🔄 重启容器..."
	docker-compose restart
	@echo "✅ 容器已重启"

# 查看日志
logs:
	docker-compose logs -f

# 查看状态
status:
	@echo "📊 容器状态:"
	@docker-compose ps
	@echo ""
	@echo "📈 资源使用:"
	@docker stats todoapp --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"

# 进入容器 shell
shell:
	docker-compose exec todoapp sh

# 清理
clean: stop
	@echo "🧹 清理容器和镜像..."
	docker-compose down -v --rmi all
	docker system prune -f
	@echo "✅ 清理完成"

# 备份数据库（经 sqlite3 .backup 生成一致性快照，避免直接 cat 活库丢失 WAL 数据）
db-backup:
	@mkdir -p backup
	@ts=$$(date +%Y%m%d_%H%M%S); \
	echo "💾 备份数据库..."; \
	docker-compose exec -T todoapp sh -c "sqlite3 /app/data/todoapp.db \".backup /tmp/todoapp_backup.db\" && cat /tmp/todoapp_backup.db" > backup/todoapp_$${ts}.db; \
	echo "✅ 备份完成: backup/todoapp_$${ts}.db"

# 恢复数据库（经 sqlite3 .restore 应用一致性快照，避免直接覆盖活库被旧 WAL 破坏）
db-restore:
	@echo "⚠️  警告：这将覆盖当前数据库！"
	@read -p "请输入备份文件路径: " backup_path; \
	cat $$backup_path | docker-compose exec -T todoapp sh -c "cat > /tmp/restore.db && sqlite3 /app/data/todoapp.db \".restore /tmp/restore.db\" && rm -f /tmp/restore.db"; \
	echo "✅ 恢复完成"
	@echo "✅ 数据库已恢复"
	@make restart

# 健康检查
health:
	@echo "🏥 健康检查..."
	@curl -s http://localhost:8080/api/v1/health | jq .

# 生产环境部署
deploy:
	@echo "🚀 部署到生产环境..."
	@echo "⚠️  确保已设置环境变量！"
	docker-compose -f docker-compose.yml up -d

# 开发环境启动
dev:
	@echo "🛠️  启动开发环境..."
	@export ENVIRONMENT=development && docker-compose up

# 测试
test:
	@echo "🧪 运行测试（需设置 JWT_SECRET）..."
	JWT_SECRET=test-secret-key-for-unit-tests-1234567890 go test -race -tags sqlite_fts5 ./...

# 查看数据库
db-shell:
	docker-compose exec todoapp sh -c "sqlite3 /app/data/todoapp.db"

# 重新构建并启动
rebuild:
	@echo "🔄 重新构建并启动..."
	docker-compose up -d --build
	@echo "✅ 完成"

# 查看日志（最近 100 行）
logs-tail:
	docker-compose logs --tail=100 todoapp

# 实时监控
monitor:
	watch -n 2 'docker stats todoapp --no-stream'
