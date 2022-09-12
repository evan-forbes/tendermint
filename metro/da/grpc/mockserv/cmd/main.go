package main

import (
	"flag"
	"log"
	"net"
	"strconv"

	grpcda "github.com/tendermint/tendermint/metro/da/grpc"
	"github.com/tendermint/tendermint/metro/da/grpc/mockserv"
)

func main() {
	conf := grpcda.DefaultConfig

	flag.IntVar(&conf.Port, "port", conf.Port, "listening port")
	flag.StringVar(&conf.Host, "host", "0.0.0.0", "listening address")
	flag.Parse()

	lis, err := net.Listen("tcp", conf.Host+":"+strconv.Itoa(conf.Port))
	if err != nil {
		log.Panic(err)
	}
	log.Println("Listening on:", lis.Addr())
	srv := mockserv.GetServer(conf, nil)
	if err := srv.Serve(lis); err != nil {
		log.Println("error while serving:", err)
	}
}
