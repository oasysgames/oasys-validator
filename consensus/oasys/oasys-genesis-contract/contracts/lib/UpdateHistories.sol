// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

import { IEnvironment } from "../IEnvironment.sol";

/**
 * @title UpdateHistories
 */
library UpdateHistories {
    function set(
        uint256[] storage epochs,
        uint256[] storage values,
        uint256 nextEpoch,
        uint256 value
    ) internal {
        extend(epochs, values, nextEpoch);
        values[epochs.length - 1] = value;
    }

    function add(
        uint256[] storage epochs,
        uint256[] storage values,
        uint256 nextEpoch,
        uint256 value
    ) internal {
        extend(epochs, values, nextEpoch);
        values[epochs.length - 1] += value;
    }

    function sub(
        uint256[] storage epochs,
        uint256[] storage values,
        uint256 nextEpoch,
        uint256 value
    ) internal returns (uint256) {
        extend(epochs, values, nextEpoch);

        uint256 length = epochs.length;
        uint256 balance = values[length - 1];
        value = value <= balance ? value : balance;
        if (value > 0) {
            values[length - 1] -= value;
        }
        return value;
    }

    function find(
        uint256[] storage epochs,
        uint256[] storage values,
        uint256 epoch
    ) internal view returns (uint256) {
        uint256 length = epochs.length;
        if (length == 0 || epochs[0] > epoch) return 0;
        if (epochs[length - 1] <= epoch) return values[length - 1];
        uint256 idx = sBinarySearch(epochs, epoch, 0, length);
        return values[idx];
    }

    function find(
        uint256[] memory epochs,
        IEnvironment.EnvironmentValue[] memory values,
        uint256 epoch
    ) internal pure returns (IEnvironment.EnvironmentValue memory) {
        uint256 length = epochs.length;
        if (epochs[length - 1] <= epoch) return values[length - 1];
        uint256 idx = mBinarySearch(epochs, epoch, 0, length);
        return values[idx];
    }

    function extend(
        uint256[] storage epochs,
        uint256[] storage values,
        uint256 nextEpoch
    ) internal {
        uint256 length = epochs.length;
        if (length == 0) {
            epochs.push(nextEpoch);
            values.push();
            return;
        }

        uint256 lastEpoch = epochs[length - 1];
        if (lastEpoch != nextEpoch) {
            epochs.push(nextEpoch);
            values.push(values[length - 1]);
        }
    }

    function sBinarySearch(
        uint256[] storage epochs,
        uint256 epoch,
        uint256 head,
        uint256 tail
    ) internal view returns (uint256) {
        if (head == tail) return tail - 1;
        uint256 center = (head + tail) / 2;
        if (epochs[center] > epoch) return sBinarySearch(epochs, epoch, head, center);
        if (epochs[center] < epoch) return sBinarySearch(epochs, epoch, center + 1, tail);
        return center;
    }

    function mBinarySearch(
        uint256[] memory epochs,
        uint256 epoch,
        uint256 head,
        uint256 tail
    ) internal pure returns (uint256) {
        if (head == tail) return tail - 1;
        uint256 center = (head + tail) / 2;
        if (epochs[center] > epoch) return mBinarySearch(epochs, epoch, head, center);
        if (epochs[center] < epoch) return mBinarySearch(epochs, epoch, center + 1, tail);
        return center;
    }
}
