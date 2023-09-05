// SPDX-License-Identifier: MIT
pragma solidity 0.8.12;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract WOAS is ERC20 {
    event Deposit(address indexed sender, uint256 amount);
    event Withdrawal(address indexed recipient, uint256 amount);

    // solhint-disable-next-line no-empty-blocks
    constructor() ERC20("Wrapped OAS", "WOAS") {}

    function deposit() external payable {
        require(msg.value > 0, "value is zero.");
        _mint(msg.sender, msg.value);
        emit Deposit(msg.sender, msg.value);
    }

    function withdraw(uint256 amount) external {
        _withdraw(amount, msg.sender);
    }

    function withdraw(uint256 amount, address recipient) external {
        _withdraw(amount, recipient);
    }

    function _withdraw(uint256 amount, address recipient) internal {
        require(amount > 0, "amount is zero.");
        require(balanceOf(msg.sender) >= amount, "over amount");
        _burn(msg.sender, amount);

        // solhint-disable-next-line avoid-low-level-calls
        (bool success, ) = recipient.call{ value: amount }("");
        require(success, "OAS transfer failed.");

        emit Withdrawal(recipient, amount);
    }
}
