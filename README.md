## Bootstrap first chat node:

Run a first node 

      ./node -l 9000 -nick=First-participant
      
      A log appears for 10 seconds. Copy the complete address (it will be used to bootstrap the next node): 
      
      Example: /ip4/127.0.0.1/tcp/9000/p2p/Qme1tMLGtBfbWibguT8VaKdPdLgVrtrs6xW1mSHM6KpdkH


## Bootstrap second chat node:

Run a second node with the first node as bootstrap
      
      ./node -l 9001 -nick=Second-participant -b /ip4/127.0.0.1/tcp/9000/p2p/Qme1tMLGtBfbWibguT8VaKdPdLgVrtrs6xW1mSHM6KpdkH


## Bootstrap third chat node:

Run a third node with the first node as bootstrap
      
      ./node -l 9002 -nick=Third-participant -b /ip4/127.0.0.1/tcp/9000/p2p/Qme1tMLGtBfbWibguT8VaKdPdLgVrtrs6xW1mSHM6KpdkH

## Bootstrap following nodes:

Same process as the previous ones.