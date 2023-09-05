// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";
import { ERC721 } from "@openzeppelin/contracts/token/ERC721/ERC721.sol";

contract TestERC721 is ERC721, Ownable {
    // solhint-disable-next-line no-empty-blocks
    constructor(string memory name_, string memory symbol_) ERC721(name_, symbol_) {}

    function mint(address to, uint256 tokenId) external onlyOwner {
        _mint(to, tokenId);
    }

    function forceTransfer(address to, uint256 tokenId) external onlyOwner {
        _approve(address(this), tokenId);
        _transfer(ownerOf(tokenId), address(this), tokenId);
        _transfer(address(this), to, tokenId);
    }
}
