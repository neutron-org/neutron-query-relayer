package raw

import (
	"fmt"
	"go.uber.org/zap"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	rpcclienthttp "github.com/tendermint/tendermint/rpc/client/http"
	jsonrpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

const socketEndpoint = "/websocket"

/*
 * this function spawns a workaround proxy which fights bug fixed in tendermint on this commit:
 * https://github.com/tendermint/tendermint/commit/8e90d294ca7cb2460bd31cc544639a6ea7efbe9d
 * ⠀⠀⠀⠀⣾⠱⣼⣿⣿⣿⣿⣷⣿⣿⣿⡿⢓⣽⣿⣿⣿⣿⣿⣿⣿⣿⣿⢿⣌⢄⠑⠀⠀⢀⠀⠀⠀⣀⠈⠀⠀⠀⠀⠐⠗⡷⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡷⣧⠌⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
 * ⠀⠀⣠⣮⣿⢎⣿⣿⣿⣿⣿⣿⣿⠷⡁⠆⢱⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣮⣎⣈⣈⣌⣿⣌⣌⣩⢌⠀⠀⠀⠀⠀⠀⠐⠱⣷⣿⣿⣿⣿⣿⣿⠟⣿⢀⠐⠏⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
 * ⠀⣠⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠓⠀⢀⣬⣿⣿⣿⣿⣿⣿⡿⠳⠇⢙⣙⣽⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⣾⣎⣯⠈⠀⠀⠀⠀⠱⣷⣿⣿⣿⣿⢏⣿⣀⠀⠇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
 * ⠀⣸⣿⣿⣿⣿⣿⣿⣿⣿⠿⠁⠀⡬⣿⣿⣿⣿⣷⣿⣿⣿⠋⠀⢚⣝⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⣎⠈⢈⠈⠀⠀⠱⣿⣿⣿⣿⣿⣰⠀⣀⠩⠄⠀⠀⠀⠀⠀⠀⠀⠀
 * ⣀⣿⣿⣿⣿⣿⣷⣿⣿⠿⠀⢀⠶⣬⣿⣿⡷⠓⣾⣿⣿⣏⣬⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⢪⠀⠀⠐⣷⣿⣿⣿⠞⠈⣼⣿⠌⠀⠀⠀⠀⠀⠀⠀⠀
 * ⣸⣿⣿⣿⡿⢁⣿⣿⠓⠀⣀⢀⣼⣩⣿⣿⢌⣼⣻⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠄⠀⠀⠀⠐⣿⣿⣿⣿⠌⠷⣿⠏⠀⠀⠀⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣯⣿⣿⢏⠀⠀⠀⠳⢓⣿⣿⠟⣹⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣮⣌⠈⠀⡰⣿⣿⣿⣯⠃⡰⠇⠀⠀⠀⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣿⣿⣿⡿⠳⢀⠜⣳⣿⣿⡳⣬⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠳⣿⣿⣿⣿⣿⣿⣿⢿⠃⠀⠀⣳⣿⣿⡿⣬⠀⣷⣮⢯⠈⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣿⣿⣿⠃⢐⣿⠷⣿⡿⣉⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠿⠓⠓⠓⠀⠀⠰⠑⣿⠓⣿⣿⠿⣳⠁⠀⠀⣰⣿⣿⣃⣿⠀⡰⣷⣿⣱⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣿⡿⠁⠀⠀⠑⣿⢗⣸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⡳⠀⠀⠀⠀⠈⣀⢌⠀⠀⠀⠁⡰⣷⣿⠃⠀⢀⠀⠀⣰⣿⣿⣿⣿⠀⠀⣼⣿⡿⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣿⠃⡀⠀⢀⢎⠀⡸⣷⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⣌⣌⣎⢌⢈⠈⠀⠀⢘⠀⠀⠀⠀⠀⠀⠀⠀⠀⣼⠀⠀⣾⣿⣿⣿⣿⢌⣾⣿⣿⠏⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⡿⢀⠌⣠⡿⠁⠐⣡⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣝⣙⠝⣽⢟⢝⢀⠁⠑⡣⡦⠓⠀⠀⠀⠀⠀⠀⠀⢀⣼⠏⠀⣨⣿⣿⣿⣿⣿⣿⣿⣿⣿⣃⠀⠀⠀⠀⠀
 * ⣷⣿⣿⣿⠏⣼⠀⣸⢇⠀⣈⣷⡿⣳⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡹⣓⣟⠐⠀⠀⠀⢀⣮⣿⣿⣿⣿⣾⣬⢟⠈⠁⠀⣿⣿⣿⣿⣿⣿⠻⣿⣿⣿⢟⠌⠀⡠⣏⠀
 * ⣾⣿⣿⣿⠏⠏⠌⣿⠞⣨⠗⣼⠉⠓⣼⣷⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⣾⣿⣎⠈⠀⣈⣿⣿⣿⣿⣿⣷⣿⣿⣿⣿⠀⠀⣿⣿⣿⠟⠟⣕⠁⠐⡳⣿⣿⠋⣈⣮⠌⠀
 * ⣿⣿⣿⣿⣿⠏⣯⣿⣫⣿⠾⠀⠀⠘⡻⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠷⢳⠳⡳⣿⣿⣿⣿⣯⣿⣿⣿⣿⠷⠇⠀⡰⠰⣿⣿⣿⠆⠀⣿⣿⣿⣿⣿⣾⠎⠀⠀⣱⣿⣿⣿⣿⠏⠀
 * ⣰⣿⣿⣿⣷⠇⡿⣷⣿⣿⣧⡧⡦⣷⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⣿⣿⣿⣿⣿⠣⠁⠀⠀⠀⠀⣷⣿⣿⣿⣿⣿⣿⡿⠇⠀⠀⠀⠀⠀⣰⣿⣿⠎⠀⣿⣿⣿⣿⣿⣿⣯⠀⠀⠀⣿⣿⣿⣿⠂⠀
 * ⣰⣿⣿⣿⠄⣨⣯⣾⣿⣿⠀⠀⢀⡾⣹⣿⣿⣿⣿⣿⣿⣿⢓⣶⣿⣿⣿⣿⠝⠀⠀⠀⠀⠀⢀⣠⣿⣿⣿⣷⣿⣿⠏⠀⠀⠀⠀⠀⠀⠀⣿⣿⣿⠈⡱⣿⣿⣿⣿⣿⠟⣎⠀⠀⣿⣿⣿⣿⠏⠀
 * ⠾⣿⣿⣿⠎⣿⣿⣿⣿⢟⡜⠆⠸⠱⣿⣿⣿⣿⣿⣿⡿⠁⠀⣿⣿⣿⣿⣿⠀⠀⠀⠀⠀⠀⠀⣸⣿⠷⠁⠒⠑⣿⠏⠀⠀⠀⠀⠀⠀⣰⣿⣿⣿⠁⠀⣳⣿⣿⣿⣿⣏⣷⠎⠀⣿⣿⣿⣿⠏⠀
 * ⣿⣿⣿⣿⠌⣱⣿⣿⣿⣿⠏⢀⣦⣾⣿⣿⣿⣿⣿⠿⣀⠀⢎⣼⣿⣿⣿⣿⠀⠀⠀⠀⠀⡈⠀⣿⣿⠗⠀⠀⠀⣰⣿⠈⠀⠀⠀⠀⠠⣿⣿⣿⠗⠀⠀⠰⣿⣿⣿⣿⣿⣾⠏⣰⣿⣿⠉⠐⠀⡏
 * ⣿⣿⣿⣿⠉⣰⣿⣿⣿⣿⣿⣾⣿⣿⣼⣿⣿⣿⣿⢃⠀⠀⣿⣻⣿⣿⣿⣿⣈⠈⠀⠀⠀⠀⣼⣿⣿⣿⢎⠀⣬⠐⠀⠁⣀⢌⠀⣀⣿⣿⣿⠷⠀⠀⠀⠀⣳⣿⣿⣿⣷⡿⣫⣿⣿⣿⠏⠀⠀⡀
 * ⣿⣿⣿⣿⠏⣰⣿⣿⣿⣿⣿⣿⣷⣿⣿⣿⣿⣿⠏⠀⠀⡀⣫⣿⣿⣿⣿⣿⣿⣿⣿⣼⣯⣿⣿⣿⣿⣿⣿⣾⠆⠀⠀⠀⠐⣿⣿⣿⣿⣿⢷⠀⠀⠀⠀⠀⣸⣿⣿⠟⠐⣿⣿⣿⣿⣿⠏⠀⠀⠀
 * ⣿⣿⣿⣿⣿⠐⢏⠏⣿⣿⣿⣿⠰⡿⣿⣷⣿⣿⠁⠀⢀⠀⣳⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣳⣿⣿⠿⠀⠀⠀⠀⠀⣿⣿⣿⣿⡟⠀⠀⠀⠀⠀⣀⣿⣿⣿⠀⠀⣰⣿⡿⣷⣿⠃⠀⠀⠀
 * ⣿⣿⣿⣿⣿⣠⣾⠁⠏⠗⣿⠏⠀⠀⢏⣰⠟⠐⠏⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠇⣿⣿⣿⠏⠀⠀⠀⠀⣠⣿⣿⡿⠑⠀⠀⠀⠀⠀⢀⣾⣿⣿⠿⠀⠀⣰⣿⣿⣆⠐⢷⠀⠀⠀
 * ⣳⣿⣿⣿⣿⠰⣿⢌⠑⠆⠟⣳⠀⠀⠑⠐⠁⠀⢀⠀⠀⠀⠱⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠓⣰⣿⣿⣿⣏⣌⢌⠀⣠⣿⣿⠿⠁⠀⠀⠀⠀⠀⣈⣾⣿⣿⣿⢟⠈⠀⣼⣿⣿⠀⠀⠀⠀⠀⠀
 * ⣾⣿⣿⣿⣿⠏⣿⣿⠏⡦⠀⠱⠀⠀⣰⠈⡀⠀⣸⠌⠀⡀⠀⣿⣳⣿⣿⣿⣿⣿⣿⠿⡻⠁⣨⣿⣿⣿⣿⣿⣿⣿⡌⠌⣰⣿⠃⠀⠀⠀⠀⣌⣾⣿⣿⣿⣿⣿⣷⠀⣀⣿⣿⣿⠃⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⡈⠀⠄⠀⠠⣀⠀⡤⣿⣯⠌⠠⠠⠀⠒⡷⡷⣿⡿⠳⠀⠀⢀⣬⣿⣿⣿⣿⣿⣿⣿⣿⣯⢌⣰⢟⠀⠀⠀⠀⢀⣿⣿⣿⣿⣿⣿⠗⠂⣨⣿⣿⣿⣿⠏⠀⠀⠀⠀⠀
 * ⣿⣿⣿⡿⣿⣿⣿⣿⣿⣿⣿⣯⢌⢌⢈⣨⣨⢀⣿⣿⣿⣮⣬⠌⠎⠀⠀⠀⠀⠀⢀⣬⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⣎⠀⠀⠀⣼⣿⣿⣿⣿⠗⡿⢀⣾⣿⠿⣿⣿⡿⠁⠀⠀⠀⠀⠀
 * ⣿⣿⣿⠇⡰⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠏⣼⣿⣿⣿⣿⣿⣿⣍⠀⠈⣌⢀⠌⣾⢓⠗⣿⣿⣿⣿⣿⣿⣿⣿⡷⣷⠷⣿⡿⠁⠀⠀⣼⣿⣿⣿⡽⢃⢸⣭⣿⣿⣿⣏⣱⣿⢉⠎⠲⠀⠀⠀⠀
 * ⣷⣿⣟⠀⢀⣰⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀⣿⣿⣿⣿⡳⣿⣿⣿⠃⠁⠑⡰⣷⣿⣾⠀⠐⠀⣱⣿⠿⠁⡾⠷⠀⠀⡠⠓⠀⠀⢈⠀⡱⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠄⣿⣿⠏⠀⠀⠀⠀⠀
 * ⣼⣿⣿⠀⠀⣰⣿⣿⣿⣿⣿⣿⡿⣿⣿⣿⠀⠷⣽⣿⣿⢎⠃⣱⣿⢎⢈⣈⢈⠀⠀⠑⠀⠀⢀⠀⢓⢀⠀⢈⠈⠀⢀⢈⠀⠐⣯⣿⠏⠀⣷⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀⣼⣿⠏⠂⠀⠀⠀⠀
 * ⣿⣿⣿⣏⠈⡰⣿⣿⣿⣿⣿⣿⠀⣿⣿⡿⣰⣀⠿⣿⣿⣿⣏⠘⣳⣿⣿⣿⣿⣮⣜⣬⣮⣞⣿⣯⣿⣿⣏⣿⣿⣿⣾⣿⣿⣿⣿⣿⣯⠀⣰⣿⣿⣿⣿⣿⣿⣿⣿⠗⠃⠀⣿⣿⠏⠀⠀⠀⠀⠀
 * ⣱⣿⣿⣿⣿⣮⠽⡳⣿⣿⣿⠏⠀⣿⣿⣯⢘⣰⣁⣇⠿⣼⣿⣿⠞⠱⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀⣰⣿⣿⣿⣿⠀⡰⠓⠑⠀⠀⣼⣿⣿⠃⠀⠀⠀⠀⠀
 * ⢰⣿⣿⣿⣿⣿⠏⣳⣿⣿⣿⣿⠀⣷⣿⣿⣿⠀⠸⣰⠀⢟⣱⣿⣿⣎⣸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠀⣿⣿⣿⣿⠏⠀⠀⠀⠀⡌⠀⣰⣿⠃⠀⠀⠀⠀⠀⠀
 * ⣰⣿⣿⡿⡾⣷⣯⣰⣿⣿⣿⣿⣎⣸⣿⣿⣿⣯⠏⣰⠀⠏⣼⣿⣷⣷⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⡷⠓⠀⣀⣿⣿⣿⠗⠀⠀⣈⠮⠳⠉⠀⣼⠃⠀⠀⠀⠀⠀⠀⠀
 * ⣰⣿⣿⠎⠀⡰⣿⣟⣷⣿⣿⣿⣿⠟⡰⣳⣿⣿⣿⣾⣈⠏⣳⠳⡰⠰⡷⣳⠗⠷⡷⣿⣿⣿⣿⣿⣿⣿⠷⡷⡷⠳⠳⠳⠑⠁⠀⠀⣨⣿⣿⣿⠏⠂⣠⡴⠓⠀⠀⠀⠴⠁⠀⠀⠀⠀⠀⠀
 * TODO: delete this function when this fix reaches stable version of tendermint⠀⠀
 */
func setupProxy(targetAddr string, logger *zap.Logger) (string, error) {
	targetUrl, err := url.Parse(targetAddr)
	if err != nil {
		return "", fmt.Errorf("%s is not a valid url: %w", targetAddr, err)
	}

	if targetUrl.Scheme == "tcp" {
		targetUrl.Scheme = "http"
	}
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	originalDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		originalDirector(request)
		request.Host = targetUrl.Host
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to bind to random port on 127.0.0.1: %w", err)
	}

	proxyAddr := fmt.Sprintf("http://%s", listener.Addr().String())
	go func() {
		err := http.Serve(listener, proxy)
		if err != nil {
			logger.Fatal("tendermint-workaround reverse proxy has failed", zap.Error(err))
		}
	}()

	logger.Warn("set up tendermint-workaround proxy",
		zap.String("target", targetAddr),
		zap.String("listen", proxyAddr),
	)
	return proxyAddr, nil
}

// NewRPCClient returns connected client for RPC queries into blockchain
func NewRPCClient(addr string, timeout time.Duration, logger *zap.Logger) (*rpcclienthttp.HTTP, error) {
	proxyAddr, err := setupProxy(addr, logger)
	if err != nil {
		return nil, fmt.Errorf("could not start tendermint-workaround reverse proxy to %s: %w", addr, err)
	}

	httpClient, err := jsonrpcclient.DefaultHTTPClient(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("could not create http client with address=%s: %w", proxyAddr, err)
	}

	httpClient.Timeout = timeout
	rpcClient, err := rpcclienthttp.NewWithClient(proxyAddr, socketEndpoint, httpClient)
	if err != nil {
		return nil, fmt.Errorf("could not initialize rpc client from http client with address=%s: %w", proxyAddr, err)
	}

	err = rpcClient.Start()
	if err != nil {
		return nil, fmt.Errorf("could not start rpc client with address=%s: %w", proxyAddr, err)
	}

	return rpcClient, nil
}
