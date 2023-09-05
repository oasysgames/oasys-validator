// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract TestERC20 is ERC20 {
    // solhint-disable-next-line no-empty-blocks
    constructor() ERC20("name", "symbol") {}

    function mint() external payable {
        _mint(msg.sender, msg.value);
    }
}
