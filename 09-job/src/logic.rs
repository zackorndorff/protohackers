use std::collections::{BTreeSet, HashMap};
use std::error::Error;
use std::sync::{Arc, Mutex};
use tokio::sync::Notify;

use crate::parser::{Id, Job, Priority};

pub(crate) type SState = Arc<Mutex<State>>;

pub(crate) struct State {
    pub customers_served: u32,
    queues: HashMap<String, BTreeSet<Job>>,
    id_to_queue: HashMap<u32, String>,
    active_jobs: HashMap<u32, Job>,
    next_id: Id,
    notifications: Vec<Arc<Notify>>,
}

impl State {
    pub fn new() -> Self {
        State {
            customers_served: 0,
            queues: HashMap::new(),
            id_to_queue: HashMap::new(),
            active_jobs: HashMap::new(),
            next_id: 0,
            notifications: vec![],
        }
    }
    pub fn request_notification(&mut self, notifier: Arc<Notify>) {
        self.notifications.push(notifier);
    }
    pub fn serve_customer(&mut self) {
        self.customers_served += 1;
    }
    pub fn next_id(&mut self) -> Id {
        let old_id = self.next_id;
        self.next_id += 1;
        old_id
    }
    pub fn add_job(&mut self, queue: String, mut job: Job) -> Id {
        let id = self.next_id();
        job.id = id;
        self.add_with_id(queue, job);
        id
    }
    fn notify_changes(&mut self, _queue: String) {
        for notify in &self.notifications {
            notify.notify_one();
        }
        self.notifications.clear();
    }

    fn add_with_id(&mut self, queue: String, job: Job) {
        let heap = self.queues.entry(queue.clone()).or_insert_with(|| BTreeSet::new());
        self.id_to_queue.insert(job.id, queue.clone());
        heap.insert(job);
        self.notify_changes(queue.clone());
    }

    pub fn find_job(&mut self, queues: &[String]) -> Option<(Job, String)> {
        let mut best: Option<(Priority, &str)> = None;
        for queue in queues {
            match self.peek_queue(queue) {
                None => {},
                Some(priority) => {
                    best = match best {
                        None => {
                            Some((priority, queue))
                        },
                        Some(best) => {
                            if best.0 >= priority {
                                Some(best)
                            } else {
                                Some((priority, queue))
                            }
                        },
                    }
                }
            }
        }
        match best {
            None => None,
            Some((_, queue)) => Some((self.take_job(queue).unwrap(), queue.to_string())),
        }
    }

    fn peek_queue(&self, queue: &str) -> Option<Priority> {
        let queue = self.queues.get(queue)?;
        Some(queue.last()?.priority)
    }

    pub fn take_job(&mut self, queue: &str) -> Option<Job> {
        let heap = self.queues.get_mut(queue)?;
        let job = heap.pop_last()?;
        self.id_to_queue.remove(&job.id);
        self.active_jobs.insert(job.id, job.clone());
        Some(job)
    }

    pub fn delete_job(&mut self, id: Id) -> Result<(), Box<dyn Error>> {
        match self.id_to_queue.remove(&id) {
            Some(queue) => {
                self.queues.get_mut(&queue).unwrap().retain(|k| k.id != id);
                Ok(())
            },
            None => {
                self.active_jobs.remove(&id).ok_or("job not found")?;
                Ok(())
            },
        }
    }

    pub fn abort_job(&mut self, id: Id, queue: String) -> Result<(), Box<dyn Error>> {
        let job = self.active_jobs.remove(&id).ok_or("job not found")?;
        self.add_with_id(queue, job);
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn testit() {
        let mut state = State::new();
        state.add_job("foo".to_owned(), Job{ priority: 5,
            id: 42,
            data: Box::new(json!{{}})});
        state.add_job("foo".to_owned(), Job{ priority: 1,
            id: 43,
            data: Box::new(json!{{}})});
        let job = state.take_job("foo").unwrap();
        assert_eq!(job.priority, 5);
        let job = state.take_job("foo").unwrap();
        assert_eq!(job.priority, 1);
    }

    #[test]
    fn testit_delete() {
        let mut state = State::new();
        let id1 = state.add_job("foo".to_owned(), Job{ priority: 5,
            id: 0,
            data: Box::new(json!{{}})});
        state.add_job("foo".to_owned(), Job{ priority: 1,
            id: 0,
            data: Box::new(json!{{}})});
        state.delete_job(id1).unwrap();
        let job = state.take_job("foo").unwrap();
        assert_eq!(job.priority, 1);
    }
}
