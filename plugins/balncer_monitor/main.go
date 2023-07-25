package main

import (
	"flag"
	"fmt"
	"github.com/rs/zerolog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	pluginpb "github.com/dsrvlabs/vatz-proto/plugin/v1"
	"github.com/dsrvlabs/vatz/sdk"
	"golang.org/x/net/context"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	defaultAddr       = "127.0.0.1"
	defaultPort       = 5001
	defaultMinBalance = 0.2
	pluginName        = "vatz-plugin-flow-balance-monitor"
	methodName        = "FlowGetBalance"
)

var (
	addr   string
	port   int
	minBal float64
)

func init() {
	flag.StringVar(&addr, "addr", defaultAddr, "IP Address(e.g. 0.0.0.0, 127.0.0.1)")
	flag.IntVar(&port, "port", defaultPort, "Port number,(default: 5001)")
	flag.Float64Var(&minBal, "min", defaultMinBalance, "Minimum Balance that must be in account,(default: 2.0)")
	flag.Parse()
}

func main() {
	p := sdk.NewPlugin(pluginName)
	p.Register(pluginFeature)

	ctx := context.Background()
	if err := p.Start(ctx, addr, port); err != nil {
		log.Info().Str("module", "plugin").Msg("exit")
	}
}

func pluginFeature(info, option map[string]*structpb.Value) (sdk.CallResponse, error) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	state := pluginpb.STATE_SUCCESS
	severity := pluginpb.SEVERITY_INFO

	cmd := "cd /mnt/flow ; ./boot-tools/bootstrap check-machine-account --access-address access.mainnet.nodes.onflow.org:9000 -o ./bootstrap | grep balance"
	contentMSG := ""
	cmdOutput, cmdError := runCommand(cmd)
	if cmdError != nil {
		state = pluginpb.STATE_FAILURE
		severity = pluginpb.SEVERITY_ERROR
		log.Error().
			Str(methodName, "Error to get a Flow balance").
			Msg(pluginName)
	}

	f := strings.Split(cmdOutput, " ")
	if len(f) > 1 {
		balance, numErr := strconv.ParseFloat(f[len(f)-1], 64)
		if numErr != nil {
			state = pluginpb.STATE_FAILURE
			severity = pluginpb.SEVERITY_ERROR
			balance = 0.00
			log.Error().
				Str(methodName, "Parsing Error to get Balance").
				Msg(pluginName)
		}
		if state == pluginpb.STATE_SUCCESS {
			if balance < minBal {
				severity = pluginpb.SEVERITY_CRITICAL
				s := fmt.Sprintf("%f", minBal)
				contentMSG = "Current balance:" + fmt.Sprintf("%.8f", balance) + " is lower than " + s + "FLOW." + "\n" + "Please, add more balance (recommended balance is 0.25FLOW)."
				log.Warn().Str(methodName, "CRITICAL: "+contentMSG).Msg(pluginName)
			} else {
				contentMSG = "Current Balance is " + fmt.Sprintf("%.8f", balance) + "FLOW."
				log.Info().
					Str(methodName, "INFO: "+contentMSG).
					Msg(pluginName)
			}
		}
	}

	ret := sdk.CallResponse{
		FuncName:   methodName,
		Message:    contentMSG,
		Severity:   severity,
		State:      state,
		AlertTypes: []pluginpb.ALERT_TYPE{pluginpb.ALERT_TYPE_DISCORD},
	}
	return ret, nil
}

func runCommand(cmd string) (string, error) {
	stdOutput := ""
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Error().
			Str(methodName, "Fail to get connected number of peers").
			Msg(pluginName)
		return stdOutput, err
	}
	outputFinal := strings.TrimSpace(string(out))
	stdOutput = outputFinal
	return stdOutput, nil

}
