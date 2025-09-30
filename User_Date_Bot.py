from datetime import datetime, timedelta
import discord
import math
import yaml

# Load the config file and get the bot token and channel ID
with open('config.yml', 'r') as f:
    config = yaml.safe_load(f)
bot_token = config['BOT_TOKEN']
channel_id = config['CHANNEL_ID']
recent_join_channel_id = config['RECENT_JOIN_CHANNEL_ID']
allowed_role_id = config['PERMS_ROLE_ID']

# Load the dates file and get the list of dates to match
with open('dates.yml', 'r') as f:
    dates = yaml.safe_load(f)
match_dates = dates

# Define the intents for your bot
intents = discord.Intents.default()
intents.message_content = True

# Enable specific intents if necessary
intents.members = True

# Create an instance of the client with the intents parameter
client = discord.Client(intents=intents)

# Define a function to check if the user's createdAt date matches any dates in the list
def check_user_created_at(user):
    user_created_at_date = user.created_at.date()  # Assume this is the user's creation date as a date object
    user_created_at_datetime = datetime.combine(user_created_at_date, datetime.min.time())

    for date_str in match_dates:
        date = datetime.strptime(date_str, '%Y-%m-%d').date()
        start_datetime = datetime.combine(date, datetime.min.time())
        end_datetime = datetime.combine(date, datetime.max.time())

        # Check if `user_created_at_datetime` is within a 1 day span of `date`
        if start_datetime - timedelta(days=1) <= user_created_at_datetime <= end_datetime + timedelta(days=1):
            return 1
    else:
        # Check if `user_created_at_datetime` is within a 1 day span of `date`
        if user_created_at_datetime >= datetime.now() - timedelta(days=1):
            return 2

    return 0
    

# Define an event listener for when a user joins the server
@client.event
async def on_member_join(member):
    
    result = check_user_created_at(member)
    if result == 1:
        a_timedelta = member.created_at.replace(tzinfo=None) - datetime(1970, 1, 1)
        seconds = math.floor(a_timedelta.total_seconds())

        # If the user's createdAt date matches a date in the list, send a message to the specified channel
        channel = client.get_channel(channel_id)
        await channel.send(f"""
⠀
:warning: **SUSPICIOUS JOIN**, new user <@!{member.id}> (**{member.name}#{member.discriminator}**, `{member.id}`):
Account creation date **<t:{seconds}:D>** matches that of known alternate accounts used by ban evaders; all dates defined in master list.
*(No action taken, awaiting manual review — take appropriate action if necessary.)*"""
        )
    elif result == 2:
        a_timedelta = member.created_at.replace(tzinfo=None) - datetime(1970, 1, 1)
        seconds = math.floor(a_timedelta.total_seconds())

        recent_join_channel = client.get_channel(recent_join_channel_id)
        await recent_join_channel.send(f"""
⠀
:new: **NEW ACCOUNT**, new user <@!{member.id}> (**{member.name}#{member.discriminator}**, `{member.id}`):
Account creation date **<t:{seconds}:D>** (<t:{seconds}:R>) is less than **24 hours old**.""")


@client.event
async def on_message(message):
    if message.author == client.user:
        return
    
    args = message.content.split()

    if len(args) <= 0:
        return

    # Check if the message is a command to add a date
    if args[0] == "~add_date":
        # Check if the user has the allowed role
        if allowed_role_id in [role.id for role in message.author.roles]:
            # Parse the date string from the message content
            date_str = args[1]
            try:
                date = datetime.strptime(date_str, '%Y-%m-%d').date()
            except ValueError:
                await message.channel.send(f'Invalid date format. Please use the format YYYY-MM-DD')
                return

            # Add the date to the `match_dates` list and save to YAML file
            match_dates.append(date_str)
            with open('dates.yml', 'w') as file:
                yaml.safe_dump(match_dates, file)

            await message.channel.send(f'Date {date_str} added successfully')

    if args[0] == "~remove_date" or args[0] == "~delete_date":
        # Check if the user has the allowed role
        if allowed_role_id in [role.id for role in message.author.roles]:
            # Get date to remove
            date_to_remove = message.content.split()[1]

            # Remove date from list
            print(f"{date_to_remove} in {match_dates}")
            if date_to_remove in match_dates:
                match_dates.remove(date_to_remove)

                # Save updated list to file
                with open("dates.yml", "w") as f:
                    yaml.dump(match_dates, f)

                await message.channel.send(f"{date_to_remove} has been removed from the list of dates")

    if args[0] == "~add_dates_between":
        args = message.content.split()
        if len(args) == 3:
            try:
                start = datetime.strptime(args[1], '%Y-%m-%d').date()
                end = datetime.strptime(args[2], '%Y-%m-%d').date()

                dates = []
                current = start
                while current <= end:
                    dates.append(current)
                    current += timedelta(days=1)
                for date in dates:
                    if str(date) not in match_dates:
                        match_dates.append(str(date))
                        match_dates.sort()
                with open('dates.yml', 'w') as file:
                    yaml.dump(match_dates, file)
                await message.channel.send(f"Successfully added dates between {start} and {end}")
            except Exception as e:
                print(e)
                await message.channel.send("Please provide a start and end date in the format YYYY-MM-DD YYYY-MM-DD")
        else:
            await message.channel.send("Please provide a start and end date in the format YYYY-MM-DD YYYY-MM-DD")

        start = message.content

    if args[0] == "~remove_dates_between" or args[0] == "~delete_dates_between":
        args = message.content.split()
        if len(args) == 3:
            try:
                start = datetime.strptime(args[1], '%Y-%m-%d').date()
                end = datetime.strptime(args[2], '%Y-%m-%d').date()

                dates = []
                current = start
                while current <= end:
                    dates.append(current)
                    current += timedelta(days=1)
                for date in dates:
                    if str(date) in match_dates:
                        match_dates.remove(str(date))
                with open('dates.yml', 'w') as file:
                    yaml.dump(match_dates, file)
                await message.channel.send(f"Successfully removed dates between {start} and {end}")
            except Exception as e:
                print(e)
                await message.channel.send("Please provide a start and end date in the format YYYY-MM-DD YYYY-MM-DD")
        else:
            await message.channel.send("Please provide a start and end date in the format YYYY-MM-DD YYYY-MM-DD")

        start = message.content

    if message.content.startswith("~dates"):
        # Check if the user has the allowed role
        if allowed_role_id in [role.id for role in message.author.roles]:
            # Sort the dates
            match_dates.sort()

            # Initialize the groups list
            groups = []
            current_group = []

            for date_str in match_dates:
                date = datetime.strptime(date_str, "%Y-%m-%d")

                if not current_group:
                    current_group.append(date)
                elif date - current_group[-1] <= timedelta(days=1):
                    current_group.append(date)
                else:
                    groups.append(current_group)
                    current_group = [date]

            if current_group:
                groups.append(current_group)

            response = ""

            # Print the groups
            for group in groups:
                # If the group only contains one date, print it on its own
                if len(group) == 1:
                    response += f"{group[0].strftime('%Y-%m-%d')}\n"
                # Otherwise, print the first and last dates in the group with an arrow in between
                else:
                    response += f"{group[0].strftime('%Y-%m-%d')} ➜ {group[-1].strftime('%Y-%m-%d')}\n"


            await message.channel.send(response)

# Start the Discord client
client.run(bot_token)

def __main__():
    print("User Creation Date Reporter Bot started!")
