// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

/**
 * @title Math
 */
library Math {
    function percent(
        uint256 numerator,
        uint256 denominator,
        uint256 precision
    ) internal pure returns (uint256) {
        uint256 numerator_ = numerator * 10**(precision + 1);
        return ((numerator_ / denominator) + 5) / 10;
    }

    function share(
        uint256 principal,
        uint256 numerator,
        uint256 denominator,
        uint256 precision
    ) internal pure returns (uint256) {
        return (principal * percent(numerator, denominator, precision)) / (10**precision);
    }
}
