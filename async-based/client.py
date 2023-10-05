import asyncio
import socket

async def udp_client():
    host = '127.0.0.1'
    port = 8888

    # Create a UDP socket
    client_socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

    try:
        while True:
            message = input("Enter a message (or 'exit' to quit): ")
            if message.lower() == 'exit':
                break

            # Send the message to the server
            client_socket.sendto(message.encode(), (host, port))

            # Receive a response from the server
            data, server_addr = client_socket.recvfrom(1024)
            print(f"Received from {server_addr}: {data.decode()}")

    except KeyboardInterrupt:
        pass
    finally:
        client_socket.close()

if __name__ == "__main__":
    asyncio.run(udp_client())
