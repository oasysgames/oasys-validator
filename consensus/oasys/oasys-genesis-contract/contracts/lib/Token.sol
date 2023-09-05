// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";

// msg.value and amount did not match.
error AmountMismatched();

// Amount is not zero.
error NotZeroAmount();

// OAS or ERC20 transfer failed.
error TransferFailed(Token.Type token);

// Unknown token type.
error UnknownToken();

library Token {
    /**
     * Type of tokens.
     *
     * OAS  - Native token of Oasys Blockchain
     * wOAS - Wrapped OAS
     * sOAS - Stakable OAS
     */
    enum Type {
        OAS,
        wOAS,
        sOAS
    }

    // Wrapped OAS
    IERC20 public constant WOAS = IERC20(0x5200000000000000000000000000000000000001);
    // Stakable OAS
    IERC20 public constant SOAS = IERC20(0x5200000000000000000000000000000000000002);

    /**
     * Receives Native or ERC20 tokens.
     * @param token Type of token to receive.
     * @param from Address of token holder.
     * @param amount Amount of token to receive.
     */
    function receives(
        Type token,
        address from,
        uint256 amount
    ) internal {
        if (token == Type.OAS) {
            if (msg.value != amount) revert AmountMismatched();
        } else {
            if (msg.value != 0) revert NotZeroAmount();
            bool success = _getERC20(token).transferFrom(from, address(this), amount);
            if (!success) revert TransferFailed(token);
        }
    }

    /**
     * Transfers Native or ERC20 tokens.
     * @param token Type of token to transfer.
     * @param to Address of token recipient.
     * @param amount Amount of token to transfer.
     */
    function transfers(
        Type token,
        address to,
        uint256 amount
    ) internal {
        bool success;
        if (token == Type.OAS) {
            // solhint-disable-next-line avoid-low-level-calls
            (success, ) = to.call{ value: amount }("");
        } else {
            success = _getERC20(token).transfer(to, amount);
        }
        if (!success) {
            revert TransferFailed(token);
        }
    }

    function _getERC20(Type token) private pure returns (IERC20) {
        if (token == Type.wOAS) {
            return WOAS;
        } else if (token == Type.sOAS) {
            return SOAS;
        }
        revert UnknownToken();
    }
}
