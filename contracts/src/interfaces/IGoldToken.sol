// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { IERC20Metadata } from "@openzeppelin/contracts/token/ERC20/extensions/IERC20Metadata.sol";
import { IERC20Permit } from "@openzeppelin/contracts/token/ERC20/extensions/IERC20Permit.sol";

/// @title IGoldToken
/// @notice 1 GOLD = 1 gram of 99.99% physical gold. 18 decimals. ERC-20 + permit + pausable.
/// @dev Mint/burn restricted to authorised controller addresses only. All transfers
///      are gated through ComplianceRegistry.canTransfer().
interface IGoldToken is IERC20, IERC20Metadata, IERC20Permit {
    /// @notice Active compliance registry address.
    function complianceRegistry() external view returns (address);

    /// @notice Authorised mint controller — only this address may call mint().
    function mintController() external view returns (address);

    /// @notice Authorised burn controller — only this address may call burnFrom().
    function burnController() external view returns (address);

    /// @notice Token issuance by controller.
    function mint(address to, uint256 amount, bytes2 jurisdiction) external;

    /// @notice Token redemption burn by controller (on behalf of user).
    /// @dev User must approve the controller via ERC-20 approve; controller pull-burns.
    function burnFrom(address from, uint256 amount) external;

    /// @notice Emergency pause (PAUSER_ROLE only).
    function pause() external;

    /// @notice Unpause (Treasury only).
    function unpause() external;

    /// @notice Pause state.
    function paused() external view returns (bool);

    // Administration
    function setComplianceRegistry(address newRegistry) external;
    function setMintController(address newController) external;
    function setBurnController(address newController) external;

    event ComplianceRegistryUpdated(address indexed oldAddr, address indexed newAddr);
    event MintControllerUpdated(address indexed oldAddr, address indexed newAddr);
    event BurnControllerUpdated(address indexed oldAddr, address indexed newAddr);
    event Minted(address indexed to, uint256 amount, bytes2 jurisdiction);
    event Burned(address indexed from, uint256 amount);
}
