import os
import logging
import re
from typing import List, Dict, Any, Tuple, Optional, Union

from telegram import Bot, Update, MessageOriginChannel
from telegram.error import TelegramError, Forbidden
from telegram.ext import Application, CommandHandler, ContextTypes, MessageHandler, filters

from dotenv import load_dotenv

# Configure logging
logging.basicConfig(
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s", level=logging.INFO
)
logger = logging.getLogger(__name__)

# Load environment variables
load_dotenv()
TELEGRAM_BOT_TOKEN = os.getenv("TG_BOT_TOKEN")
if not TELEGRAM_BOT_TOKEN:
    raise ValueError("TG_BOT_TOKEN environment variable is not set")


async def extract_chat_id_from_invite(bot: Bot, invite_link: str) -> Optional[str]:
    """
    Try to extract a chat ID from an invite link by joining the channel.
    This only works if the bot is allowed to join via this invite.
    """
    try:
        # Extract the invite part from the link
        invite_hash = None
        if '+' in invite_link:
            invite_hash = invite_link.split('+')[-1].split('?')[0]
        elif 'joinchat/' in invite_link:
            invite_hash = invite_link.split('joinchat/')[-1].split('?')[0]

        if not invite_hash:
            return None

        # Try to join the chat - this only works if the bot doesn't need admin approval
        chat = await bot.join_chat(invite_hash)
        return str(chat.id)
    except TelegramError as e:
        logger.error(f"Failed to join via invite link: {e}")
        return None


async def check_specific_channel(bot: Bot, identifier: str) -> Dict[str, Any]:
    """
    Check if the bot is a member and admin in a specific channel.
    Identifier can be a username, chat_id, or invite link.

    Returns dict with:
    - exists: Whether channel exists
    - is_member: Whether bot is a member
    - is_admin: Whether bot is an admin
    - channel_id: ID of the channel if found
    - linked_chat_id: ID of the linked discussion group if available
    - has_linked_chat_admin: Whether bot is admin in the linked chat
    """
    result = {
        "exists": False,
        "is_member": False,
        "is_admin": False,
        "channel_id": None,
        "linked_chat_id": None,
        "has_linked_chat_admin": False
    }

    chat_id = None

    # Check if it's already a chat ID (starts with -)
    if identifier.startswith('-'):
        chat_id = identifier
    # Check if it's an invite link
    elif 't.me/' in identifier and ('+' in identifier or 'joinchat/' in identifier):
        chat_id = await extract_chat_id_from_invite(bot, identifier)
        if not chat_id:
            result["error"] = "Could not join via invite link. The bot may need admin approval to join."
            return result

    try:
        # Try to get channel info (using username or resolved chat_id)
        channel_info = await bot.get_chat(chat_id or identifier)
        result["exists"] = True
        result["channel_id"] = str(channel_info.id)
        result["channel_title"] = channel_info.title

        # Bot must be member to get chat info
        result["is_member"] = True

        # Check if bot is admin
        bot_id = (await bot.get_me()).id
        admins = await bot.get_chat_administrators(channel_info.id)
        result["is_admin"] = any(admin.user.id == bot_id for admin in admins)

        # Check for linked discussion group
        if channel_info.linked_chat_id:
            result["linked_chat_id"] = str(channel_info.linked_chat_id)
            try:
                linked_admins = await bot.get_chat_administrators(channel_info.linked_chat_id)
                result["has_linked_chat_admin"] = any(admin.user.id == bot_id for admin in linked_admins)
            except TelegramError:
                # Can't check admin status in linked chat
                pass

    except Forbidden:
        # Bot knows the channel exists but isn't a member
        result["exists"] = True
        result["error"] = "Bot doesn't have access to this channel"
    except TelegramError as e:
        logger.error(f"Error checking channel {identifier}: {e}")
        result["error"] = f"Error: {str(e)}"

    return result


async def forward_handler(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle forwarded messages from channels to check their status"""
    print(update.message.forward_origin.type)
    if not update.message or update.message.forward_origin.type != "channel":
        return

    # Message was forwarded from a channel
    forwarded_chat = update.message.forward_origin.chat
    chat_id = str(forwarded_chat.id)

    bot = context.bot
    result = await check_specific_channel(bot, chat_id)

    channel_name = forwarded_chat.title or "Unknown channel"

    if not result["is_member"]:
        await update.message.reply_text(f"❌ Bot is not a member of \"{channel_name}\" (ID: {chat_id})")
    elif not result["is_admin"]:
        await update.message.reply_text(f"⚠️ Bot is a member but not an admin in \"{channel_name}\" (ID: {chat_id})")
    else:
        admin_status = f"✅ Bot is an admin in the channel \"{channel_name}\""
        if result["linked_chat_id"]:
            if result["has_linked_chat_admin"]:
                admin_status += " and its linked discussion group"
            else:
                admin_status += " but NOT in its linked discussion group"

        await update.message.reply_text(
            f"{admin_status}\n"
            f"Channel ID: {result['channel_id']}\n"
            f"Discussion Group ID: {result['linked_chat_id'] or 'None'}"
        )


async def group_message_handler(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    print(update.message.chat.id, update.message.text)
    """Handle messages from groups that are linked to channels"""
    if not update.message or not update.message.chat or not update.message.chat.linked_chat_id:
        return

    linked_chat_id = str(update.message.chat.linked_chat_id)
    bot = context.bot
    result = await check_specific_channel(bot, linked_chat_id)

    if not result["is_member"]:
        await update.message.reply_text(f"❌ Bot is not a member of the linked discussion group (ID: {linked_chat_id})")
    elif not result["is_admin"]:
        await update.message.reply_text(f"⚠️ Bot is a member but not an admin in the linked discussion group (ID: {linked_chat_id})")
    else:
        await update.message.reply_text(f"✅ Bot is an admin in the linked discussion group (ID: {linked_chat_id})")


async def check_channel_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle the /checkchannel command"""
    if update.message is None:
        return

    if not context.args or len(context.args) == 0:
        await update.message.reply_text(
            "Please provide a channel username, invite link, or ID:\n"
            "/checkchannel @channel_name\n"
            "/checkchannel https://t.me/+abc123\n"
            "/checkchannel -1001234567890\n\n"
            "For private channels, either:\n"
            "1. Forward a message from the channel to me\n"
            "2. Add me to the channel and use its invite link or ID"
        )
        return

    channel_identifier = context.args[0]

    # Normalize the channel username if provided with @
    if channel_identifier.startswith("https://t.me/") and not (
            '+' in channel_identifier or 'joinchat/' in channel_identifier):
        username = channel_identifier.split("https://t.me/")[1].split("/")[0]
        channel_identifier = f"@{username}"
    elif not channel_identifier.startswith(('@', '-', 'https://')):
        channel_identifier = f"@{channel_identifier}"

    bot = context.bot
    result = await check_specific_channel(bot, channel_identifier)

    if "error" in result:
        await update.message.reply_text(f"❌ {result['error']}")
    elif not result["exists"]:
        await update.message.reply_text(f"❌ Channel not found or inaccessible")
    elif not result["is_member"]:
        await update.message.reply_text(f"❌ Bot is not a member of the channel")
    elif not result["is_admin"]:
        await update.message.reply_text(f"⚠️ Bot is a member but not an admin in the channel")
    else:
        channel_title = result.get("channel_title", "the channel")
        admin_status = f"✅ Bot is an admin in {channel_title}"
        if result["linked_chat_id"]:
            if result["has_linked_chat_admin"]:
                admin_status += " and its linked discussion group"
            else:
                admin_status += " but NOT in its linked discussion group"

        await update.message.reply_text(
            f"{admin_status}\n"
            f"Channel ID: {result['channel_id']}\n"
            f"Discussion Group ID: {result['linked_chat_id'] or 'None'}"
        )


async def start_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle the /start command"""
    if update.message is None:
        return

    help_text = (
        "👋 Hi! I'm a channel access checking bot.\n\n"
        "To check a private channel, you can:\n"
        "1️⃣ Forward a message from the channel to me\n"
        "2️⃣ Add me to the channel, then send me the invite link or channel ID\n\n"
        "For public channels, just use:\n"
        "/checkchannel @channel_name\n\n"
        "/help - For more information"
    )
    await update.message.reply_text(help_text)


async def help_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle the /help command"""
    if update.message is None:
        return

    help_text = (
        "I can check if I'm a member and admin in a Telegram channel.\n\n"
        "For public channels:\n"
        "/checkchannel @username\n\n"
        "For private channels, you can:\n"
        "1. Forward any message from the channel to me\n"
        "2. Add me to the channel first, then use one of these:\n"
        "   /checkchannel -100123456789 (channel ID)\n"
        "   /checkchannel https://t.me/+abcdef... (invite link)\n\n"
        "Note: I can only check channels where I'm already a member."
    )
    await update.message.reply_text(help_text)


if __name__ == "__main__":
    # Initialize application
    application = Application.builder().token(TELEGRAM_BOT_TOKEN).build()

    # Add command handlers
    application.add_handler(CommandHandler("start", start_command))
    application.add_handler(CommandHandler("help", help_command))
    application.add_handler(CommandHandler("checkchannel", check_channel_command))

    # Add handler for forwarded messages
    application.add_handler(MessageHandler(filters.FORWARDED, forward_handler))
    # Обрабатываем сообщения из групп, привязанных к каналам
    application.add_handler(MessageHandler(filters.ALL, group_message_handler))

    # Start the long polling
    logger.info("Starting bot with long polling")
    application.run_polling(allowed_updates=Update.ALL_TYPES)
