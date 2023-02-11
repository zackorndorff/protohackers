use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use tokio::net::TcpListener;
use tokio::io::{self, AsyncWriteExt, AsyncBufReadExt, BufReader, BufWriter};
use tokio::sync::Notify;

mod parser;
mod logic;

use logic::{State, SState};
use parser::{Response, Job,
Status, ResponseSpecifics,
PutRequest, GetRequest, DeleteRequest, AbortRequest, Request};
use parser::Request::*;

use parser::Id;

use std::marker::Unpin;

struct Client {
    in_progress_jobs: HashMap<Id, String>,
    state: SState,
}

impl Client {
    fn new(state: SState) -> Self {
        Client {
            in_progress_jobs: HashMap::new(),
            state,
        }
    }
}

impl Drop for Client {
    fn drop(&mut self) {
        for (job, queue) in self.in_progress_jobs.drain() {
            let mut state = self.state.lock().unwrap();
            let _ = state.abort_job(job, queue);
        }
    }
}

enum MaybeRetry<T> {
    Now(T),
    Retry(Arc<Notify>, Request),
}
use MaybeRetry::{Now, Retry};

fn handle_put_req(state: &mut State, req: PutRequest) -> MaybeRetry<Response> {
    let job = Job {
        priority: req.pri,
        data: Box::new(req.job),
        id: 0,
    };
    let id = state.add_job(req.queue, job);
    Now(Response {
        status: Status::Ok,
        rest: ResponseSpecifics::IdResponse{id},
    })
}

fn handle_get_req(state: &mut State, req: GetRequest, client: &mut Client) -> MaybeRetry<Response> {
    let job = state.find_job(&req.queues);
    match job {
        Some((job, queue)) => {
            client.in_progress_jobs.insert(job.id, queue.clone());
            Now(Response {
                status: Status::Ok,
                rest: ResponseSpecifics::JobResponse{job, queue},
            })
        },
        None => {
            if req.wait {
                let notifier = Arc::new(Notify::new());
                state.request_notification(notifier.clone());
                Retry(notifier, parser::Request::Get(req))
            } else {
                Now(Response::error(Status::NoJob))
            }
        },
    }
}

fn handle_delete_req(state: &mut State, req: DeleteRequest) -> MaybeRetry<Response> {
    match state.delete_job(req.id) {
        Ok(_) => Now(Response::error(Status::Ok)),
        Err(_) => Now(Response::error(Status::NoJob)),
    }
}

fn handle_abort_req(state: &mut State, req: AbortRequest, client: &mut Client) -> MaybeRetry<Response> {
    let queue = match client.in_progress_jobs.remove(&req.id) {
        None => {
            return Now(Response::error(Status::Error));
        },
        Some(queue) => queue,
    };
    match state.abort_job(req.id, queue) {
        Err(_) => Now(Response::error(Status::NoJob)),
        Ok(_) => Now(Response::error(Status::Ok)),
    }
}

async fn send_response(socket: &mut (impl AsyncWriteExt + Unpin), resp: Response) -> io::Result<()> {
    let mut data_str = serde_json::to_string(&resp).unwrap();
    println!("Sending: {}", data_str);
    data_str.push('\n');
    socket.write_all(data_str.as_bytes()).await?;
    socket.flush().await?;
    Ok(())
}

async fn handle_connection(state: SState, socket: tokio::net::TcpStream) -> io::Result<()>{
    let mut client = Client::new(state.clone());
    let socket = BufReader::new(socket);
    let mut socket = BufWriter::new(socket);
    let mut line = String::new();
    loop {
        line.clear();
        if socket.read_line(&mut line).await? == 0 {
            break;
        }
        println!("reading line: {}", line);
        let req = serde_json::from_str(&line);
        println!("got request: {:?}", req);
        let mut req = match req {
            Ok(req) => req,
            Err(e) => {
                println!("bad request: {:?}", e);
                let resp = Response::error(Status::Error);
                send_response(&mut socket, resp).await?;
                continue;
            },
        };
        //socket.write_all(format!("{:?}\n", req).as_bytes()).await;
        let response = loop {
            let response;
            {
                let mut state = state.lock().unwrap();
                response = match req {
                    Put(putreq) => handle_put_req(&mut state, putreq),
                    Get(getreq) => {
                        let res = handle_get_req(&mut state, getreq, &mut client);
                        res
                    },
                    Delete(delreq) => handle_delete_req(&mut state, delreq),
                    Abort(abreq) => handle_abort_req(&mut state, abreq, &mut client),
                };
                drop(state);
            }
            match response {
                Now(response) => { break response; },
                Retry(notifier, request) => {
                    req = request;
                    notifier.notified().await;
                },
            };
        };
        send_response(&mut socket, response).await?;
        socket.flush().await?;
    }
    Ok(())
}

#[tokio::main]
async fn main() -> io::Result<()> {
    let state = Arc::new(Mutex::new(State::new()));
    let listener = TcpListener::bind("0.0.0.0:1337").await?;

    loop {
        let (socket, _) = listener.accept().await?;
        println!("in loop");
        let state = state.clone();
        tokio::spawn(async move {
           if let Err(e) = handle_connection(state, socket).await {
               println!("got an error {}", e);
           }
        });
    }
}

#[cfg(test)]
mod tests {
    //use super::*;
}
