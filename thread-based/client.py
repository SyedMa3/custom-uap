import socket
import struct
import sys
import threading
import time
import random

# Define constants
MAGIC_NUMBER = 0xC461
VERSION = 1
HELLO = 0
DATA = 1
ALIVE = 2
GOODBYE = 3

HELLO_WAIT = 0
READY = 1
READY_TIMER = 2
CLOSING = 3
CLOSED = 4

TIMEOUT = 30

state = HELLO_WAIT
global seq_num
seq_num = 0

# Create a UDP socket for the client
server_host = sys.argv[1]  # Get server hostname or IP address from command line argument
server_port = int(sys.argv[2])  # Get server port number from command line argument

client_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

# Function to send a UAP message
def send_message(sock, command, session_id, data=b''):
    global seq_num
    header = struct.pack('!HBBII', MAGIC_NUMBER, VERSION, command, seq_num, session_id)
    seq_num += 1
    message = header + data
    sock.sendto(message, (server_host, server_port))

# Function to handle server responses
def receive_responses(sock, lock, state):
    sock.settimeout(TIMEOUT)
    while True:
        try:
            data, server_address = sock.recvfrom(1024)
            # print(data)

            # Parse the header
            magic, version, command, _, _ = struct.unpack('!HBBII', data[:12])

            # Check if the magic number and version are correct
            if magic != MAGIC_NUMBER or version != VERSION:
                print("Invalid message received")
                continue
            
            with lock:
                if state == HELLO_WAIT:
                    if command != 0x00:
                        print("Invalid message received")
                        return
                    
                    state = READY
                    sock.settimeout(None)
                elif state == READY:
                    if command == ALIVE:
                        continue
                elif state == READY_TIMER:
                    if command == ALIVE:
                        state = READY
                        sock.settimeout(None)
                elif state == CLOSING:
                    if command == ALIVE:
                        continue
                    else:
                        state = CLOSED
                        return
        except socket.timeout:
            print("Timeout")
            return
                

# Create a thread for receiving responses
lock = threading.Lock()
response_thread = threading.Thread(target=receive_responses, args=(client_socket,lock, state))
response_thread.daemon = True
response_thread.start()
# Main thread for sending user input to the server
session_id = random.randint(0, 2**32 - 1)
send_message(client_socket, HELLO, session_id)

while True:
    line = sys.stdin.readline().strip()
    
    if not line:
        break  # End of file
    
    
    
    with lock:
        if state == READY:
            client_socket.settimeout(TIMEOUT)
            state = READY_TIMER
        elif state == READY_TIMER:
            continue
    # print(line)
    if line == "q":
        break
    send_message(client_socket, DATA, session_id, line.encode())
    

send_message(client_socket, GOODBYE, session_id)
