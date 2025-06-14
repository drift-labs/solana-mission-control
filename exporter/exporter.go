package exporter

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Chainflow/solana-mission-control/alerter"
	"github.com/Chainflow/solana-mission-control/config"
	"github.com/Chainflow/solana-mission-control/monitor"
	"github.com/Chainflow/solana-mission-control/querier"
	"github.com/Chainflow/solana-mission-control/types"
	"github.com/Chainflow/solana-mission-control/utils"
)

const (
	httpTimeout = 5 * time.Second
)

// solanaCollector respresents a set of solana metrics
type solanaCollector struct {
	config                    *config.Config
	totalValidatorsDesc       *prometheus.Desc
	validatorActivatedStake   *prometheus.Desc
	validatorLastVote         *prometheus.Desc
	validatorRootSlot         *prometheus.Desc
	validatorDelinquent       *prometheus.Desc
	solanaVersion             *prometheus.Desc
	accountBalance            *prometheus.Desc
	slotLeader                *prometheus.Desc
	blockTime                 *prometheus.Desc
	currentSlot               *prometheus.Desc
	commission                *prometheus.Desc
	delinqentCommission       *prometheus.Desc
	validatorVote             *prometheus.Desc
	statusAlertCount          *prometheus.Desc
	ipAddress                 *prometheus.Desc
	txCount                   *prometheus.Desc
	netVoteHeight             *prometheus.Desc
	valVoteHeight             *prometheus.Desc
	voteHeightDiff            *prometheus.Desc
	valVotingStatus           *prometheus.Desc
	voteCredits               *prometheus.Desc
	networkVoteCredits        *prometheus.Desc
	networkConfirmationTime   *prometheus.Desc
	validatorConfirmationTime *prometheus.Desc
	confirmationTimeDiff      *prometheus.Desc
	// confirmed block time of network
	networkBlockTime *prometheus.Desc
	// confirmed block time of validator
	validatorBlockTime *prometheus.Desc
	// block time difference of network and validator
	blockTimeDiff      *prometheus.Desc
	voteAccBalance     *prometheus.Desc
	identityAccBalance *prometheus.Desc
	lastEpoch          *int64
	// Cache fields to reduce redundant API calls
	cachedEpochInfo    *types.EpochInfo
	cachedEpochTime    time.Time
	cachedVoteAccounts *types.GetVoteAccountsResponse
	cachedVoteAccTime  time.Time
}

// NewSolanaCollector exports solana collector metrics to prometheus
func NewSolanaCollector(cfg *config.Config) *solanaCollector {
	return &solanaCollector{
		config: cfg,
		totalValidatorsDesc: prometheus.NewDesc(
			"solana_active_validators",
			"Total number of active validators by state",
			[]string{"state"}, nil),
		validatorActivatedStake: prometheus.NewDesc(
			"solana_validator_activated_stake",
			"Activated stake per validator",
			[]string{"votekey", "pubkey"}, nil),
		validatorLastVote: prometheus.NewDesc(
			"solana_validator_last_vote",
			"Last voted slot per validator",
			[]string{"votekey", "pubkey"}, nil),
		validatorRootSlot: prometheus.NewDesc(
			"solana_validator_root_slot",
			"Root slot per validator",
			[]string{"votekey", "pubkey"}, nil),
		validatorDelinquent: prometheus.NewDesc(
			"solana_validator_delinquent",
			"Whether a validator is delinquent",
			[]string{"votekey", "pubkey"}, nil),
		solanaVersion: prometheus.NewDesc(
			"solana_node_version",
			"Node version of solana",
			[]string{"version"}, nil),
		accountBalance: prometheus.NewDesc( // check using or not
			"solana_account_balance",
			"Solana identity account balance",
			[]string{"solana_acc_balance"}, nil),
		slotLeader: prometheus.NewDesc(
			"solana_slot_leader",
			"Current slot leader",
			[]string{"solana_slot_leader"}, nil),
		currentSlot: prometheus.NewDesc(
			"solana_current_slot",
			"Current slot height",
			[]string{"solana_current_slot"}, nil,
		),
		blockTime: prometheus.NewDesc(
			"solana_block_time",
			"Current block time.",
			[]string{"solana_block_time"}, nil,
		),
		commission: prometheus.NewDesc(
			"solana_val_commission",
			"Solana validator current commission.",
			[]string{"solana_val_commission"}, nil,
		),
		delinqentCommission: prometheus.NewDesc(
			"solana_val_delinquuent_commission",
			"Solana validator delinqent commission.",
			[]string{"solana_delinquent_commission"}, nil,
		),
		validatorVote: prometheus.NewDesc(
			"solana_vote_account",
			"whether the vote account is staked for this epoch",
			[]string{"state"}, nil,
		),
		statusAlertCount: prometheus.NewDesc(
			"solana_val_alert_count",
			"Count of alerts about validator status alerting",
			[]string{"alert_count"}, nil,
		),
		ipAddress: prometheus.NewDesc(
			"solana_ip_address",
			"IP Address from clustrnode information, gossip",
			[]string{"ip_address"}, nil,
		),
		txCount: prometheus.NewDesc(
			"solana_tx_count",
			"solana transaction count",
			[]string{"solana_tx_count"}, nil,
		),
		netVoteHeight: prometheus.NewDesc(
			"solana_network_vote_height",
			"solana network vote height",
			[]string{"solana_network_vote_height"}, nil,
		),
		valVoteHeight: prometheus.NewDesc(
			"solana_validator_vote_height",
			"solana validator vote height",
			[]string{"solana_validator_vote_height"}, nil,
		),
		voteHeightDiff: prometheus.NewDesc(
			"solana_vote_height_diff",
			"solana vote height difference of validator and network",
			[]string{"solana_vote_height_diff"}, nil,
		),
		valVotingStatus: prometheus.NewDesc(
			"solana_val_status",
			"solana validator voting status i.e., voting or jailed.",
			[]string{"solana_val_status"}, nil,
		),
		voteCredits: prometheus.NewDesc(
			"solana_validator_vote_credits",
			"solana validator vote credits of previous and current epoch.",
			[]string{"type"}, nil,
		),
		networkVoteCredits: prometheus.NewDesc(
			"solana_network_vote_credits",
			"solana network average vote credits of previous and current epoch.",
			[]string{"type"}, nil,
		),
		networkBlockTime: prometheus.NewDesc(
			"solana_network_confirmed_time",
			"Confirmed Block time of network",
			[]string{"solana_network_confirmed_time"}, nil,
		),
		validatorBlockTime: prometheus.NewDesc(
			"solana_val_confirmed_time",
			"Confirmed Block time of validator",
			[]string{"solana_val_confirmed_time"}, nil,
		),
		blockTimeDiff: prometheus.NewDesc(
			"solana_confirmed_blocktime_diff",
			"Block time difference of network and validator",
			[]string{"solana_confirmed_blocktime_diff"}, nil,
		),
		voteAccBalance: prometheus.NewDesc(
			"solana_vote_account_balance",
			"Vote account balance",
			[]string{"solana_vote_acc_bal"}, nil,
		),
		identityAccBalance: prometheus.NewDesc(
			"solana_identity_account_balance",
			"Identity account balance",
			[]string{"solana_identity_acc_bal"}, nil,
		),
	}

}

// Desribe exports metrics to the channel
func (c *solanaCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.solanaVersion
	ch <- c.accountBalance
	ch <- c.totalValidatorsDesc
	ch <- c.slotLeader
	ch <- c.currentSlot
	ch <- c.commission
	ch <- c.delinqentCommission
	ch <- c.validatorVote
	ch <- c.ipAddress
	// ch <- c.StatusAlertCount
	ch <- c.txCount
	ch <- c.netVoteHeight
	ch <- c.valVoteHeight
	ch <- c.voteHeightDiff
	ch <- c.valVotingStatus
	ch <- c.networkBlockTime
	ch <- c.validatorBlockTime
	ch <- c.blockTimeDiff
	ch <- c.voteAccBalance
	ch <- c.identityAccBalance
}

// mustEmitMetrics gets the data from Current and Deliquent validator vote accounts and export metrics of validator Vote account to prometheus.
//
//	Those metrics are
//
// 1. Current validator's info
// 2. Deliquent validator's info
// 3. Curent validator node key and vote key
// 4. Validator vote account wether it is voting or not and send alert
// 5. Current validator Vote commision
// 6. Validator Activated Stake
// 7. Validator Vote Height
// 8. Network Vote Height
// 9. VOte Height difference of Validator and Network
// 10. Validator Vote Credits
// 11. Deliquent validator commision
// 12. Deliquent validatot vote account whether it voting or not and send alerts
func (c *solanaCollector) mustEmitMetrics(ch chan<- prometheus.Metric, response types.GetVoteAccountsResponse) {
	ch <- prometheus.MustNewConstMetric(c.totalValidatorsDesc, prometheus.GaugeValue,
		float64(len(response.Result.Delinquent)), "delinquent")
	ch <- prometheus.MustNewConstMetric(c.totalValidatorsDesc, prometheus.GaugeValue,
		float64(len(response.Result.Current)), "current")

	for _, account := range append(response.Result.Current, response.Result.Delinquent...) {
		if account.NodePubkey == c.config.ValDetails.PubKey {
			// ch <- prometheus.MustNewConstMetric(c.validatorActivatedStake, prometheus.GaugeValue,
			// 	float64(account.ActivatedStake), account.VotePubkey, account.NodePubkey)
			ch <- prometheus.MustNewConstMetric(c.validatorLastVote, prometheus.GaugeValue,
				float64(account.LastVote), account.VotePubkey, account.NodePubkey)
			ch <- prometheus.MustNewConstMetric(c.validatorRootSlot, prometheus.GaugeValue,
				float64(account.RootSlot), account.VotePubkey, account.NodePubkey)
		}
	}

	var epochvote float64
	var valresult float64

	// Get epoch info once and reuse it for all vote accounts
	_, err := c.getCachedEpochInfo()
	if err != nil {
		log.Printf("Error while getting epoch info : %v", err)
	}

	// Get network vote info from the response data we already have
	var netresult float64
	for _, vote := range response.Result.Current {
		if vote.NodePubkey == c.config.ValDetails.PubKey {
			netresult = float64(vote.LastVote)
			break
		}
	}

	var runningCurrentCredits, runningPreviousCredits float64
	var currentCreditsCount, previousCreditsCount int64
	// current vote account information
	for _, vote := range response.Result.Current {
		cCredits, pCredits := c.calcualteEpochVoteCredits(vote.EpochCredits)
		if cCredits != 0 && pCredits != 0 {
			runningCurrentCredits += cCredits
			runningPreviousCredits += pCredits
			currentCreditsCount++
			previousCreditsCount++
		}
		if vote.NodePubkey == c.config.ValDetails.PubKey {
			v := strconv.FormatInt(vote.Commission, 10)

			if vote.EpochVoteAccount {
				epochvote = 1
			} else {
				epochvote = 0
			}
			ch <- prometheus.MustNewConstMetric(c.validatorVote, prometheus.GaugeValue,
				epochvote, "current") // store vote account is staked or not

			ch <- prometheus.MustNewConstMetric(c.commission, prometheus.GaugeValue, float64(vote.Commission), v) // store commission

			ch <- prometheus.MustNewConstMetric(c.validatorDelinquent, prometheus.GaugeValue,
				0, vote.VotePubkey, vote.NodePubkey) // stor vote key and node key

			stake := float64(vote.ActivatedStake) / math.Pow(10, 9)
			ch <- prometheus.MustNewConstMetric(c.validatorActivatedStake, prometheus.GaugeValue,
				stake, vote.VotePubkey, vote.NodePubkey) // store activated stake

			// Check weather the validator is voting or not
			if !vote.EpochVoteAccount && vote.ActivatedStake <= 0 {
				msg := "Solana validator is NOT VOTING"
				c.AlertValidatorStatus(msg, ch)

				ch <- prometheus.MustNewConstMetric(c.valVotingStatus, prometheus.GaugeValue, 0, "Jailed")
			} else {
				msg := "Solana validator is VOTING"
				c.AlertValidatorStatus(msg, ch)

				ch <- prometheus.MustNewConstMetric(c.valVotingStatus, prometheus.GaugeValue, 1, "Voting")
			}
			valresult = float64(vote.LastVote)
			ch <- prometheus.MustNewConstMetric(c.valVoteHeight, prometheus.GaugeValue, valresult, "validator")
			ch <- prometheus.MustNewConstMetric(c.netVoteHeight, prometheus.GaugeValue, netresult, "network")
			diff := netresult - valresult
			ch <- prometheus.MustNewConstMetric(c.voteHeightDiff, prometheus.GaugeValue, diff, "vote height difference")

			// calcualte vote credits
			ch <- prometheus.MustNewConstMetric(c.voteCredits, prometheus.GaugeValue, float64(cCredits), "current")
			ch <- prometheus.MustNewConstMetric(c.voteCredits, prometheus.GaugeValue, float64(pCredits), "previous")
		}
	}

	avgCurrentCredits := runningCurrentCredits / float64(currentCreditsCount)
	avgPreviousCredits := runningPreviousCredits / float64(previousCreditsCount)
	ch <- prometheus.MustNewConstMetric(c.networkVoteCredits, prometheus.GaugeValue, avgCurrentCredits, "current")
	ch <- prometheus.MustNewConstMetric(c.networkVoteCredits, prometheus.GaugeValue, avgPreviousCredits, "previous")

	// delinquent vote account information
	for _, vote := range response.Result.Delinquent {
		if vote.NodePubkey == c.config.ValDetails.PubKey {
			v := strconv.FormatInt(vote.Commission, 10)
			// if vote.EpochVoteAccount {
			// 	epochvote = 1
			// } else {
			// 	epochvote = 0
			// }
			// ch <- prometheus.MustNewConstMetric(c.validatorVote, prometheus.GaugeValue,
			// 	epochvote, "delinquent")
			ch <- prometheus.MustNewConstMetric(c.delinqentCommission, prometheus.GaugeValue, float64(vote.Commission), v) // store delinquent commission

			// send alert if the validator is delinquent
			ch <- prometheus.MustNewConstMetric(c.validatorDelinquent, prometheus.GaugeValue,
				1, vote.VotePubkey, vote.NodePubkey)

			// Send Telegram Alert
			telegramErr := alerter.SendTelegramAlert(fmt.Sprintf("Your solana validator is in DELINQUENT state"), c.config)
			if telegramErr != nil {
				log.Printf("Error while sending vallidator status alert to telegram: %v", telegramErr)
			}

			// Send Email Alert
			emailErr := alerter.SendEmailAlert(fmt.Sprintf("Your solana validator is in DELINQUNET state"), c.config)
			if emailErr != nil {
				log.Printf("Error while sending validator status alert to email: %v", emailErr)
			}

			// Send Slack Alert
			slackErr := alerter.SendSlackAlert(fmt.Sprintf("Your solana validator is in DELINQUENT state"), c.config)
			if slackErr != nil {
				log.Printf("Error while sending validator status alert to slack: %v", slackErr)
			}
		}
	}
}

// calculateEpochVoteCredits returns epoch credits of vote account
func (c *solanaCollector) calcualteEpochVoteCredits(credits [][]int64) (float64, float64) {
	epochInfo, err := c.getCachedEpochInfo()
	if err != nil {
		log.Printf("Error while getting epoch info : %v", err)
	}

	epoch := epochInfo.Result.Epoch
	var currentCredits, previousCredits int64

	for _, c := range credits {
		if len(c) >= 3 {
			if c[0] == epoch {
				currentCredits = c[1]
				previousCredits = c[2]
				break
			}
		}
	}

	// log.Printf("Current Epoch : %d\n Current Epoch Vote Credits: %d\n Previous Epoch Vote Credits : %d\n", epoch, currentCredits, previousCredits)

	return float64(currentCredits), float64(previousCredits)
}

// AlertValidatorStatus sends validator status alerts at respective alert timings.
func (c *solanaCollector) AlertValidatorStatus(msg string, ch chan<- prometheus.Metric) {
	now := time.Now().UTC()
	currentTime := now.Format(time.Kitchen)

	var alertsArray []string

	for _, value := range c.config.RegularStatusAlerts.AlertTimings {
		t, _ := time.Parse(time.Kitchen, value)
		alertTime := t.Format(time.Kitchen)

		alertsArray = append(alertsArray, alertTime)
	}

	log.Printf("Current time : %v and alerts array : %v", currentTime, alertsArray)

	var count float64 = 0

	for _, statusAlertTime := range alertsArray {
		if currentTime == statusAlertTime {
			alreadySentAlert, _ := querier.AlertStatusCountFromPrometheus(c.config)
			if alreadySentAlert == "false" {
				telegramErr := alerter.SendTelegramAlert(msg, c.config)
				emailErr := alerter.SendEmailAlert(msg, c.config)
				slackErr := alerter.SendSlackAlert(msg, c.config)
				if telegramErr != nil {
					log.Printf("Error while sending vallidator status alert to telegram: %v", telegramErr)
				}
				if emailErr != nil {
					log.Printf("Error while sending validator status alert to email: %v", emailErr)
				}
				if slackErr != nil {
					log.Printf("Error while sending validator status alert to slack: %v", slackErr)
				}
				ch <- prometheus.MustNewConstMetric(c.statusAlertCount, prometheus.GaugeValue,
					count, "true")
				count = count + 1
			} else {
				ch <- prometheus.MustNewConstMetric(c.statusAlertCount, prometheus.GaugeValue,
					count, "false")
				return
			}
		}
	}
}

// Collect get data from methods and exports metrics to prometheus. Those metrics are
// 1. Solana version
// 2. Identity account and Vote account balance
// 3. slot Leader
// 4. Confirmed block time of validator
// 5. Confirmed block time of network
// 6. Confirmed block time difference of validator and network
// 7. IP address
// 8. Total transaction count
// 9. Get current block time and previous block time and difference of both.
func (c *solanaCollector) Collect(ch chan<- prometheus.Metric) {
	// Only collect metrics that are NOT handled by WatchSlots()
	// WatchSlots() already handles: balance, nodeHealth, epochInfo, skipRate, blockProduction

	// Vote accounts - only needed for validator-specific metrics, not for general prometheus metrics
	accs, err := monitor.GetVoteAccounts(c.config, utils.Validator)
	if err != nil {
		ch <- prometheus.NewInvalidMetric(c.totalValidatorsDesc, err)
		ch <- prometheus.NewInvalidMetric(c.validatorActivatedStake, err)
		ch <- prometheus.NewInvalidMetric(c.validatorLastVote, err)
		ch <- prometheus.NewInvalidMetric(c.validatorRootSlot, err)
		ch <- prometheus.NewInvalidMetric(c.validatorDelinquent, err)
	} else {
		c.mustEmitMetrics(ch, accs) // emit vote account metrics
	}

	// get version - this is static, low frequency call
	version, err := monitor.GetVersion(c.config)
	if version.Result.SolanaCore != "" {
		ch <- prometheus.MustNewConstMetric(c.solanaVersion, prometheus.GaugeValue, 1, version.Result.SolanaCore)
	}

	// NOTE: Removed duplicate balance calls that WatchSlots() already handles:
	// - GetIdentityBalance (WatchSlots calls this every 2 seconds -> balance.Set())
	// - GetVoteAccBalance (redundant)
	// - Multiple GetCurrentSlot calls (duplicated in block time functions)
	// - GetBlockTime calls (multiple calls per scrape causing burst)
	// - GetConfirmedBlock calls (causing "Getting Confirmed Block..." spam)
	// - GetClusterNodes (for IP address - causing "Getting Cluster Nodes..." spam)

	// The WatchSlots() function already handles these metrics every 2 seconds:
	// - balance.Set() for account balance (maps to account_balance metric)
	// - nodeHealth.Set() for node health
	// - networkEpoch.Set() and related epoch metrics
	// - skipRate metrics
	// - blockProduction metrics

	// get slot leader - keeping this as it's used by some dashboards
	leader, err := monitor.GetSlotLeader(c.config)
	if err != nil {
		ch <- prometheus.NewInvalidMetric(c.slotLeader, err)
	} else {
		if leader.Result != "" {
			ch <- prometheus.MustNewConstMetric(c.slotLeader, prometheus.GaugeValue, 1, leader.Result)
		}
	}

	// get current validator slot - single call
	slot, err := monitor.GetCurrentSlot(c.config, utils.Validator)
	if err != nil {
		log.Printf("Error while getting current slot info : %v", err)
	} else {
		cs := strconv.FormatInt(slot.Result, 10)
		ch <- prometheus.MustNewConstMetric(c.currentSlot, prometheus.GaugeValue, float64(slot.Result), cs)
	}

	// tx count - keeping this but it could be moved to WatchSlots if needed
	count, _ := monitor.GetTxCount(c.config)
	txcount := utils.NearestThousandFormat(float64(count.Result))
	ch <- prometheus.MustNewConstMetric(c.txCount, prometheus.GaugeValue, float64(count.Result), txcount)
}

// getClusterNodeInfo returns gossip address of node
func (c *solanaCollector) getClusterNodeInfo() string {
	result, err := monitor.GetClusterNodes(c.config)
	if err != nil {
		log.Printf("Error while getting cluster node information : %v", err)
	}
	var address string
	for _, value := range result.Result {
		if value.Pubkey == c.config.ValDetails.PubKey {
			// ch <- prometheus.MustNewConstMetric(c.ipAddress, prometheus.GaugeValue, 1, value.Gossip)
			address = value.Gossip
		}
	}
	return address
}

// getNetworkVoteAccountinfo returns last vote  information of  network vote account
func (c *solanaCollector) getNetworkVoteAccountinfo() float64 {
	resn, _ := monitor.GetVoteAccounts(c.config, utils.Network)
	var outN float64
	for _, vote := range resn.Result.Current {
		if vote.NodePubkey == c.config.ValDetails.PubKey {
			outN = float64(vote.LastVote)

		}
	}
	return outN
}

// get confirmed block time of network
func (c *solanaCollector) getNetworkBlockTime(slot int64) int64 {
	result, err := monitor.GetConfirmedBlock(c.config, slot, utils.Network)
	if err != nil {
		log.Printf("failed to fetch confirmed time of network, retrying: %v", err)
		// cancel()
	}
	return result.Result.BlockTime
}

// get confirmed blocktime of validator
func (c *solanaCollector) getValidatorBlockTime(slot int64) int64 {
	result, err := monitor.GetConfirmedBlock(c.config, slot, utils.Validator)
	if err != nil {
		log.Printf("failed to fetch confirmed time of network, retrying: %v", err)
		// cancel()
	}
	return result.Result.BlockTime
}

// blockTimeDiff calculate block time difference
func blockTimeDiff(bt int64, pvt int64) (float64, string) {
	t1 := time.Unix(bt, 0)
	t2 := time.Unix(pvt, 0)

	sub := t1.Sub(t2)
	diff := sub.Seconds()

	if diff < 0 {
		diff = -(diff)
	}
	s := fmt.Sprintf("%.2f", diff)

	sec, _ := strconv.ParseFloat(s, 64)

	return sec, s
}

// getCachedEpochInfo returns cached epoch info or fetches new data if cache is expired
func (c *solanaCollector) getCachedEpochInfo() (*types.EpochInfo, error) {
	// Cache for 30 seconds
	if c.cachedEpochInfo != nil && time.Since(c.cachedEpochTime) < 30*time.Second {
		return c.cachedEpochInfo, nil
	}

	epochInfo, err := monitor.GetEpochInfo(c.config, utils.Validator)
	if err != nil {
		return nil, err
	}

	c.cachedEpochInfo = &epochInfo
	c.cachedEpochTime = time.Now()
	return &epochInfo, nil
}
