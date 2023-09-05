// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

import { Constants } from "./Constants.sol";
import { Math } from "./Math.sol";
import { UpdateHistories } from "./UpdateHistories.sol";
import { Validator as ValidatorLib } from "./Validator.sol";
import { Token } from "./Token.sol";
import { IEnvironment } from "../IEnvironment.sol";
import { IStakeManager } from "../IStakeManager.sol";

/**
 * @title Staker
 */
library Staker {
    using UpdateHistories for uint256[];
    using ValidatorLib for IStakeManager.Validator;

    /********************
     * Public Functions *
     ********************/

    function stake(
        IStakeManager.Staker storage staker,
        IEnvironment environment,
        IStakeManager.Validator storage validator,
        Token.Type token,
        uint256 amount
    ) internal {
        staker.stakeUpdates[token][validator.owner].add(
            staker.stakeAmounts[token][validator.owner],
            environment.epoch() + 1,
            amount
        );
        validator.stake(environment, staker.signer, amount);
    }

    function unstake(
        IStakeManager.Staker storage staker,
        IEnvironment environment,
        IStakeManager.Validator storage validator,
        Token.Type token,
        uint256 amount
    ) internal returns (uint256) {
        uint256 epoch = environment.epoch();
        uint256 current = getStake(staker, validator.owner, token, epoch);
        uint256 next = getStake(staker, validator.owner, token, epoch + 1);

        amount = staker.stakeUpdates[token][validator.owner].sub(
            staker.stakeAmounts[token][validator.owner],
            epoch + 1,
            amount
        );
        if (amount == 0) return 0;
        validator.unstake(environment, amount);

        uint256 unstakes = amount;
        uint256 refunds;
        if (next > current) {
            refunds = next - current;
            refunds = amount < refunds ? amount : refunds;
            unstakes -= refunds;
        }
        if (unstakes > 0) {
            _addUnstakeAmount(staker, environment, token, unstakes);
        }
        if (refunds > 0) {
            Token.transfers(token, staker.signer, refunds);
        }
        return amount;
    }

    function claimRewards(
        IStakeManager.Staker storage staker,
        IEnvironment environment,
        IStakeManager.Validator storage validator,
        uint256 epochs
    ) internal {
        (uint256 rewards, uint256 lastClaim) = getRewards(staker, environment, validator, epochs);
        staker.lastClaimReward[validator.owner] = lastClaim;
        if (rewards > 0) {
            Token.transfers(Token.Type.OAS, staker.signer, rewards);
        }
    }

    function claimUnstakes(IStakeManager.Staker storage staker, IEnvironment environment) internal {
        _claimUnstakes(staker, environment, Token.Type.wOAS);
        _claimUnstakes(staker, environment, Token.Type.sOAS);
        _claimUnstakes(staker, environment, Token.Type.OAS);
    }

    /******************
     * View Functions *
     ******************/

    function getStake(
        IStakeManager.Staker storage staker,
        address validator,
        Token.Type token,
        uint256 epoch
    ) internal view returns (uint256) {
        return staker.stakeUpdates[token][validator].find(staker.stakeAmounts[token][validator], epoch);
    }

    function getRewards(
        IStakeManager.Staker storage staker,
        IEnvironment environment,
        IStakeManager.Validator storage validator,
        uint256 epochs
    ) internal view returns (uint256 rewards, uint256 lastClaim) {
        lastClaim = staker.lastClaimReward[validator.owner];
        uint256 prevEpoch = environment.epoch() - 1;
        if (epochs == 0 || epochs + lastClaim > prevEpoch) {
            epochs = prevEpoch - lastClaim;
        }

        for (uint256 i = 0; i < epochs; i++) {
            lastClaim += 1;

            uint256 _stake = getStake(staker, validator.owner, Token.Type.OAS, lastClaim) +
                getStake(staker, validator.owner, Token.Type.wOAS, lastClaim) +
                getStake(staker, validator.owner, Token.Type.sOAS, lastClaim);
            if (_stake == 0) continue;

            uint256 validatorRewards = validator.getRewardsWithoutCommissions(
                environment.findValue(lastClaim),
                lastClaim
            );
            if (validatorRewards == 0) continue;

            rewards += Math.share(
                validatorRewards,
                _stake,
                validator.getTotalStake(lastClaim),
                Constants.REWARD_PRECISION
            );
        }
    }

    function getUnstakes(
        IStakeManager.Staker storage staker,
        IEnvironment environment,
        Token.Type token
    ) internal view returns (uint256) {
        uint256 length = staker.unstakeUpdates[token].length;
        if (length == 0) return 0;

        uint256 epoch = environment.epoch();
        uint256 idx = length - 1;
        if (idx > 0 && staker.unstakeUpdates[token][idx] > epoch) {
            idx--;
        }
        if (staker.unstakeUpdates[token][idx] > epoch) return 0;

        uint256 unstakes;
        for (uint256 i = 0; i <= idx; i++) {
            unstakes += staker.unstakeAmounts[token][i];
        }
        return unstakes;
    }

    /*********************
     * Private Functions *
     *********************/

    function _addUnstakeAmount(
        IStakeManager.Staker storage staker,
        IEnvironment environment,
        Token.Type token,
        uint256 amount
    ) private {
        uint256 nextEpoch = environment.epoch() + 1;
        uint256 length = staker.unstakeUpdates[token].length;

        if (length == 0 || staker.unstakeUpdates[token][length - 1] != nextEpoch) {
            staker.unstakeUpdates[token].push(nextEpoch);
            staker.unstakeAmounts[token].push(amount);
            return;
        }
        staker.unstakeAmounts[token][length - 1] += amount;
    }

    function _claimUnstakes(
        IStakeManager.Staker storage staker,
        IEnvironment environment,
        Token.Type token
    ) private {
        uint256 unstakes = getUnstakes(staker, environment, token);
        if (unstakes == 0) return;

        uint256 length = staker.unstakeUpdates[token].length;
        if (staker.unstakeUpdates[token][length - 1] <= environment.epoch()) {
            delete staker.unstakeUpdates[token];
            delete staker.unstakeAmounts[token];
        } else {
            staker.unstakeUpdates[token] = [staker.unstakeUpdates[token][length - 1]];
            staker.unstakeAmounts[token] = [staker.unstakeAmounts[token][length - 1]];
        }

        Token.transfers(token, staker.signer, unstakes);
    }
}
