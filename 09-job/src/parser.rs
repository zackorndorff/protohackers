use std::cmp::Ordering;
use serde::{Serialize, Deserialize};
use serde_json::{json, value::Value};

pub type Priority = u32;

pub type Id = u32;

#[derive(Debug, Serialize, Clone)]
pub struct Job {
    #[serde(rename = "pri")]
    pub priority: Priority,
    pub id: Id,
    #[serde(rename = "job")]
    pub data: Box<Value>,
}

impl Ord for Job {
    fn cmp(&self, other: &Self) -> Ordering {
        match self.priority.cmp(&other.priority) {
            Ordering::Equal => self.id.cmp(&other.priority),
            o @ _ => o,
        }
    }
}

impl PartialOrd for Job {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl PartialEq for Job {
    fn eq(&self, other: &Self) -> bool {
        self.priority.eq(&other.priority) && self.id.eq(&other.id)
    }
}

impl Eq for Job {}

#[derive(Debug, Serialize, Deserialize)]
pub struct PutRequest {
    pub queue: String,
    pub job: Value,
    pub pri: u32,
}

fn default_wait() -> bool {
    false
}

#[derive(Debug, Serialize, Deserialize)]
pub struct GetRequest {
    pub queues: Vec<String>,
    #[serde(default = "default_wait")]
    pub wait: bool,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct DeleteRequest {
    pub id: u32,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct AbortRequest {
    pub id: u32,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "request")]
#[serde(rename_all = "lowercase")]
pub enum Request {
    Put(PutRequest),
    Get(GetRequest),
    Delete(DeleteRequest),
    Abort(AbortRequest),
}

#[derive(Debug, Serialize, Clone)]
pub enum Status {
    #[serde(rename = "no-job")]
    NoJob,
    #[serde(rename = "ok")]
    Ok,
    #[serde(rename = "error")]
    Error,
}

#[derive(Debug, Serialize, Clone)]
#[serde(untagged)]
pub enum ResponseSpecifics {
    JobResponse{
        #[serde(flatten)]
        job: Job,
        queue: String,
    },
    IdResponse{id: Id},
    Empty,
}

fn is_empty(resp: &ResponseSpecifics) -> bool {
    if let ResponseSpecifics::Empty = resp {
        true
    } else {
        false
    }
}

#[derive(Debug, Serialize, Clone)]
pub struct Response {
    pub status: Status,
    #[serde(flatten)]
    #[serde(skip_serializing_if = "is_empty")]
    pub rest: ResponseSpecifics
}

impl Response {
    pub fn error(status: Status) -> Self {
        Response {
            status,
            rest: ResponseSpecifics::Empty,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn stuff() {
        let resp = Response {
            status: Status::Ok,
            rest: ResponseSpecifics::IdResponse{id: 42},
        };
        assert_eq!(r#"{"status":"ok","id":42}"#, 
            serde_json::to_string(&resp).unwrap());

        let job = Job{ priority: 5,
            id: 42,
            data: Box::new(json!{{}})};
        let resp = Response {
            status: Status::Ok,
            rest: ResponseSpecifics::JobResponse{job, queue: "hi".to_string()},
        };
        assert_eq!(r#"{"status":"ok","pri":5,"id":42,"job":{},"queue":"hi"}"#,
                   serde_json::to_string(&resp).unwrap());
        let resp = Response {
            status: Status::Ok,
            rest: ResponseSpecifics::Empty,
        };
        assert_eq!(r#"{"status":"ok"}"#,
                   serde_json::to_string(&resp).unwrap());
    }

    fn smoke_test(s: &str) -> Request {
        let deserialized: Request = serde_json::from_str(s).unwrap();
        deserialized
    }

    macro_rules! smoke_test {
        (@recurse $name:ident: $value:expr) => {
            #[test]
            fn $name() {
                smoke_test($value);
            }
        };
        ($($name: ident: $value:expr,)*) => {
            $(
                smoke_test!{@recurse $name: $value}
            )*
        };
    }

    smoke_test! {
        put: r#"{"request":"put","queue":"queue1","job":{"foo": 1, "bar": 5},"pri":123}"#,
        get_wait: r#"{"request":"get","queues":["queue1","queue2","queue3"],"wait":true}"#,
        get: r#"{"request":"get","queues":["queue1","queue2","queue3"]}"#,
        delete: r#"{"request":"delete","id":12345}"#,
        abort: r#"{"request":"abort","id":12345}"#,
    }
}
