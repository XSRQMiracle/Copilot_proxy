from abc import ABC, abstractmethod


class BaseAdapter(ABC):

    @abstractmethod
    def handle_request(self, path):
        raise NotImplementedError
