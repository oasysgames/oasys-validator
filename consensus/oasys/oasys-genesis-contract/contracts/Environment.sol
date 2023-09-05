// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

import { System } from "./System.sol";
import { IEnvironment } from "./IEnvironment.sol";
import { UpdateHistories } from "./lib/UpdateHistories.sol";
import { EnvironmentValue as EnvironmentValueLib } from "./lib/EnvironmentValue.sol";

// Not executable in the last block of epoch.
error OnlyNotLastBlock();

// Epoch must be the future.
error PastEpoch();

/**
 * @title Environment
 * @dev The Environment contract has parameters for proof-of-stake.
 */
contract Environment is IEnvironment, System {
    using UpdateHistories for uint256[];
    using EnvironmentValueLib for EnvironmentValue;

    /*************
     * Variables *
     *************/

    // Update history of environment values
    uint256[] public updates;
    EnvironmentValue[] public values;

    /****************************
     * Functions for Validators *
     ****************************/

    /**
     * @inheritdoc IEnvironment
     */
    function initialize(EnvironmentValue memory initialValue) external onlyCoinbase initializer {
        initialValue.startBlock = 0;
        initialValue.startEpoch = 1;
        _updateValue(initialValue);
    }

    /**
     * @inheritdoc IEnvironment
     */
    function updateValue(EnvironmentValue memory newValue) external onlyCoinbase {
        if (isLastBlock()) revert OnlyNotLastBlock();
        if (newValue.startEpoch <= epoch()) revert PastEpoch();

        EnvironmentValue storage next = _getNext();
        if (next.started(block.number)) {
            newValue.startBlock = next.nextStartBlock(newValue);
        } else {
            newValue.startBlock = _getCurrent().nextStartBlock(newValue);
        }
        _updateValue(newValue);
    }

    /******************
     * View Functions *
     ******************/

    /**
     * @inheritdoc IEnvironment
     */
    function epoch() public view returns (uint256) {
        EnvironmentValue storage next = _getNext();
        return next.started(block.number) ? next.epoch() : _getCurrent().epoch();
    }

    /**
     * @inheritdoc IEnvironment
     */
    function isFirstBlock() external view returns (bool) {
        return (block.number) % value().epochPeriod == 0;
    }

    /**
     * @inheritdoc IEnvironment
     */
    function isLastBlock() public view returns (bool) {
        return (block.number + 1) % value().epochPeriod == 0;
    }

    /**
     * @inheritdoc IEnvironment
     */
    function value() public view returns (EnvironmentValue memory) {
        EnvironmentValue storage next = _getNext();
        return next.started(block.number) ? next : _getCurrent();
    }

    /**
     * @inheritdoc IEnvironment
     */
    function nextValue() external view returns (EnvironmentValue memory) {
        EnvironmentValue memory current = value();
        EnvironmentValue storage next = _getNext();
        uint256 nextStartBlock = current.startBlock + (epoch() - current.startEpoch + 1) * current.epochPeriod;
        return next.started(nextStartBlock) ? next : current;
    }

    /**
     * @inheritdoc IEnvironment
     */
    function findValue(uint256 _epoch) external view returns (EnvironmentValue memory) {
        return updates.find(values, _epoch);
    }

    /*********************
     * Private Functions *
     *********************/

    /**
     * Returns the current (or previous) environment value.
     * @return Environment value.
     */
    function _getCurrent() internal view returns (EnvironmentValue storage) {
        uint256 length = values.length;
        if (length == 1) {
            return values[0];
        }
        return values[length - 2];
    }

    /**
     * Returns the next (or current) environment value.
     * @return Environment value.
     */
    function _getNext() internal view returns (EnvironmentValue storage) {
        return values[values.length - 1];
    }

    /**
     * Validate the new environment value and if there are no problems, save to storage.
     * @param _value New environment value.
     */
    function _updateValue(EnvironmentValue memory _value) private {
        _value.validate();

        uint256 length = updates.length;
        if (length == 0 || values[length - 1].started(block.number)) {
            updates.push(_value.startEpoch);
            values.push(_value);
        } else {
            updates[length - 1] = _value.startEpoch;
            values[length - 1] = _value;
        }
    }
}
