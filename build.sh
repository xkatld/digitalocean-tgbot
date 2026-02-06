
DIST_DIR="dist"
CONFIG_FILE="config.yaml"

go mod tidy

rm -rf $DIST_DIR
mkdir -p $DIST_DIR

build_and_pack() {
    OS=$1
    ARCH=$2
    OUT_NAME="Linux-${ARCH}"
    
    echo "正在构建: $OUT_NAME ..."
    
    CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build -o $DIST_DIR/$OUT_NAME main.go
    
    if [ $? -eq 0 ]; then
        cd $DIST_DIR
        cp "../$CONFIG_FILE" .
        tar -czf "${OUT_NAME}.tar.gz" "$OUT_NAME" "$CONFIG_FILE"
        rm "$OUT_NAME" "$CONFIG_FILE"
        cd ..
        echo "[正确] 已打包: ${OUT_NAME}.tar.gz"
    else
        echo "[错误] 构建失败: $OUT_NAME"
    fi
}

build_and_pack "linux" "amd64"
build_and_pack "linux" "arm64"

echo "构建完成，输出目录: $DIST_DIR"
